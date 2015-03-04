package ownet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
)

type OW struct {
	address string
	conn    net.Conn
	hdrbuf  []byte
	sg      int32
	sync.Mutex
}

type header struct {
	Version int32
	Payload int32
	Type    int32
	Flags   int32
	Size    int32
	Offset  int32
}

// OWNet message types
const (
	MsgError       uint32 = iota
	MsgNop                = iota
	MsgRead               = iota
	MsgWrite              = iota
	MsgDir                = iota
	MsgSize               = iota
	MsgPresence           = iota
	MsgDirAll             = iota
	MsgGet                = iota
	MsgDirAllSlash        = iota
	MsgGetSlash           = iota
)

type OWErr int32

func (e OWErr) Error() string {
	return fmt.Sprintf("owserver returned error %v", int32(e))
}

// Regexp matching device identifiers as shown in owserver root directory
var DeviceRegex = regexp.MustCompile("[0-9A-F]{2}\\.[0-9A-F]{12}")

// Create a new OWNet client object. Supply owserver address in "host:port" format.
// Connection will be established on first request.
func New(address string) *OW {
	if address == "" {
		address = "127.0.0.1:4304"
	}
	return &OW{
		address: address,
		sg:      0x102, // some magic flags value
	}
}

func (ow *OW) dial() (err error) {
	ow.conn, err = net.DialTimeout("tcp", ow.address, time.Second*30)
	return
}

// Close connection to owserver.
func (ow *OW) Close() {
	if ow.conn != nil {
		ow.conn.Close()
		ow.conn = nil
	}
}

func (ow *OW) msgRead(payload []byte) (hdr header, n int, err error) {
	if err = binary.Read(ow.conn, binary.BigEndian, &hdr); err != nil {
		return
	}
	//log.Printf("<- %+v\n", hdr)
	if hdr.Payload > 0 && len(payload) >= int(hdr.Payload) {
		n, err = ow.conn.Read(payload[:hdr.Payload])
	}
	//log.Printf("<- n:%v payload:%v\n", n, string(payload))
	return
}

func (ow *OW) msgWrite(hdr header, payload []byte) (err error) {
	if ow.conn == nil {
		err = ow.dial()
		if err != nil {
			return
		}
	}
	var buf bytes.Buffer
	//log.Printf("-> %+v\n", hdr)
	//log.Printf("-> payload: %v\n", string(payload))
	binary.Write(&buf, binary.BigEndian, hdr)
	buf.Write(payload)
	for ; buf.Len() > 0 && err == nil; _, err = buf.WriteTo(ow.conn) {
	}
	return
}

// Get listing of specified directory.
// Returns array with directory items names and error if any.
func (ow *OW) Dir(path string) (items []string, err error) {
	ow.Lock()
	defer ow.Unlock()

	err = ow.dial()
	if err != nil {
		return
	}
	defer ow.Close()

	ret := make([]byte, 4096, 4096)
	hdr := header{
		Version: 0,
		Payload: int32(len(path) + 1),
		Type:    MsgDirAll,
		Flags:   ow.sg,
		Size:    int32(len(ret)),
	}
	err = ow.msgWrite(hdr, append([]byte(path), 0))
	if err != nil {
		return
	}
	hdr, _, err = ow.msgRead(ret)
	if err != nil {
		return
	}
	if hdr.Type != 0 {
		return nil, OWErr(hdr.Type)
	}
	return strings.Split(string(ret), ","), err
}

// Read owserver file with path starting from offset into data.
// Returns number of read bytes and error if any.
func (ow *OW) Read(path string, offset int, data []byte) (n int, err error) {
	ow.Lock()
	defer ow.Unlock()

	err = ow.dial()
	if err != nil {
		return
	}
	defer ow.Close()

	hdr := header{
		Version: 0,
		Payload: int32(len(path) + 1),
		Type:    MsgRead,
		Flags:   ow.sg,
		Size:    int32(len(data)),
		Offset:  int32(offset),
	}
	err = ow.msgWrite(hdr, append([]byte(path), 0))
	if err != nil {
		return
	}
	hdr, n, err = ow.msgRead(data)
	if err != nil {
		return
	}
	if hdr.Type < 0 {
		err = OWErr(hdr.Type)
		return
	}
	return
}

// Write data to owserver file at path starting from offset.
// Returns nil on success, otherwise error.
func (ow *OW) Write(path string, offset int, data []byte) (err error) {
	ow.Lock()
	defer ow.Unlock()

	err = ow.dial()
	if err != nil {
		return
	}
	defer ow.Close()

	hdr := header{
		Version: 0,
		Payload: int32(len(path) + 1 + len(data)),
		Type:    MsgWrite,
		Flags:   ow.sg,
		Size:    int32(len(data)),
		Offset:  int32(offset),
	}
	err = ow.msgWrite(hdr, append(append([]byte(path), 0), data...))
	if err != nil {
		return
	}
	hdr, _, err = ow.msgRead(nil)
	if err != nil {
		return
	}
	if hdr.Type < 0 {
		err = OWErr(hdr.Type)
		return
	}
	return
}

// Get list of present devices on the bus. Devices identified with DeviceRegex.
// Returns array of device identifiers and error if any.
func (ow *OW) ListDevices() (devs []string, err error) {
	var dir []string
	dir, err = ow.Dir("/")
	if err != nil {
		return
	}
	for _, item := range dir {
		dev := DeviceRegex.FindString(item)
		if dev != "" {
			devs = append(devs, dev)
		}
	}
	return
}

// Get value of attribute attr of the device.
// Returns attribute value and error if any.
func (ow *OW) GetAttr(device, attr string) (string, error) {
	buf := make([]byte, 16, 16)
	if n, err := ow.Read(fmt.Sprintf("/%s/%s", device, attr), 0, buf); err != nil {
		return "", err
	} else {
		return string(buf[:n]), nil
	}
}

// Set value of attribute attr of the device to value.
// Returns nil on success, error otherwise.
func (ow *OW) SetAttr(device, attr, value string) error {
	return ow.Write(fmt.Sprintf("/%s/%s", device, attr), 0, []byte(value))
}

// Get type of the device. Equal to GetAttr(device, "type).
// Refer to owfs documentation for possible device types and their corresponding
// sets of attributes.
// Returns device type and error if any.
func (ow *OW) GetType(device string) (string, error) {
	return ow.GetAttr(device, "type")
}
