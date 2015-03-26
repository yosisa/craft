package mux

import (
	"net"
	"testing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type MuxSuite struct{}

var _ = Suite(&MuxSuite{})

func (s *MuxSuite) TestServer(c *C) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, IsNil)
	m := &Mux{}
	m.Handle(0x00, HandlerFunc(func(c net.Conn) {
		b := make([]byte, 1024)
		n, _ := c.Read(b)
		c.Write(b[:n])
		c.Close()
	}))
	m.Handle(0x01, HandlerFunc(func(c net.Conn) {
		b := make([]byte, 1024)
		n, _ := c.Read(b)
		c.Write(b[:n])
		c.Write(b[:n])
		c.Close()
	}))
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go m.HandleTCP(conn)
		}
	}()

	in := make([]byte, 1024)
	read := func(c net.Conn) {
		in = in[:1024]
		n, _ := c.Read(in)
		in = in[:n]
	}

	conn, err := net.Dial("tcp", ln.Addr().String())
	c.Assert(err, IsNil)
	conn = NewClient(conn, 0x00)
	conn.Write([]byte("Hello"))
	read(conn)
	c.Assert(string(in), Equals, "Hello")

	conn, err = net.Dial("tcp", ln.Addr().String())
	c.Assert(err, IsNil)
	conn = NewClient(conn, 0x01)
	conn.Write([]byte("Hello"))
	read(conn)
	c.Assert(string(in), Equals, "HelloHello")
}
