package ownet

import (
	"testing"
)

const (
	srv  = "192.168.0.10:4304"
	attr = "/3A.BEE71B000000/PIO.B"
)

func TestDir(t *testing.T) {
	ow := New(srv)

	dir, err := ow.Dir("/")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("dir: %+v\n", dir)
}

func TestRead(t *testing.T) {
	ow := New(srv)

	buf := make([]byte, 16, 16)
	n, err := ow.Read(attr, 0, buf)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("data: %+v\n", buf[:n])
}

func TestWrite(t *testing.T) {
	ow := New(srv)

	err := ow.Write(attr, 0, []byte("1"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestListDevices(t *testing.T) {
	ow := New(srv)

	devs, err := ow.ListDevices()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("devs: %+v\n", devs)
}
