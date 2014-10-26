package ownet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

type OW struct {
	address string
	conn    net.Conn
	hdrbuf  []byte
	sg      int32
}

type header struct {
	Version int32
	Payload int32
	Type    int32
	Flags   int32
	Size    int32
	Offset  int32
}

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

var reDevice = regexp.MustCompile("[0-9A-F]{2}\\.[0-9A-F]{12}")

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

func (ow *OW) Close() {
	ow.conn.Close()
	ow.conn = nil
}

func (ow *OW) msgRead(payload []byte) (hdr header, n int, err error) {
	if ow.conn == nil {
		err = ow.dial()
		if err != nil {
			return
		}
	}
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

func (ow *OW) Dir(path string) (items []string, err error) {
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

func (ow *OW) Read(path string, offset int, data []byte) (n int, err error) {
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

func (ow *OW) Write(path string, offset int, data []byte) (err error) {
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

func (ow *OW) ListDevices() (devs []string, err error) {
	var dir []string
	dir, err = ow.Dir("/")
	if err != nil {
		return
	}
	for _, item := range dir {
		dev := reDevice.FindString(item)
		if dev != "" {
			devs = append(devs, dev)
		}
	}
	return
}

func (ow *OW) GetAttr(device, attr string) (string, error) {
	buf := make([]byte, 16, 16)
	if n, err := ow.Read(fmt.Sprintf("/%s/%s", device, attr), 0, buf); err != nil {
		return "", err
	} else {
		return string(buf[:n]), nil
	}
}

func (ow *OW) SetAttr(device, attr, value string) error {
	return ow.Write(fmt.Sprint("/%s/%s", device, attr), 0, []byte(value))
}

func (ow *OW) GetType(device string) (string, error) {
	return ow.GetAttr(device, "type")
}
