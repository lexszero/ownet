package ownet

import (
	"testing"
)

const (
	srv = "192.168.0.10:4304"
	attr = "/3A.BEE71B000000/PIO.B"
)

func TestDir(t *testing.T) {
	ow := New(srv)
	defer ow.Close()

	dir, err := ow.Dir("/")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("dir: %+v\n", dir)
}

func TestRead(t *testing.T) {
	ow := New(srv)
	defer ow.Close()

	buf := make([]byte, 16, 16)
	n, err := ow.Read(attr, 0, buf)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("data: %+v\n", buf[:n])
}

func TestWrite(t *testing.T) {
	ow := New(srv)
	defer ow.Close()

	err := ow.Write(attr, 0, []byte("1"))
	if err != nil {
		t.Fatal(err)
	}
}
