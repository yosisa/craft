package rpc

import (
	"encoding/binary"
	"log"
	"net"
	"net/rpc"

	"github.com/yosisa/craft/mux"
)

func AllocStream(c *rpc.Client, addr string) (id uint32, conn net.Conn, err error) {
	var resp AllocResponse
	if err = c.Call("StreamConn.Alloc", Empty{}, &resp); err != nil {
		return
	}
	id = resp.ID
	if conn, err = mux.DialTimeout("tcp", addr, chanNewStream, dialTimeout); err != nil {
		return
	}
	err = binary.Write(conn, binary.BigEndian, id)
	return
}

func PullImage(addrs []string, image string) error {
	p := newProgress()
	go p.show()
	CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		id, sc, err := AllocStream(c, addr)
		if err == nil {
			p.add(sc, addr)
		} else {
			log.Print(err)
		}
		req := PullImageRequest{Image: image, StreamID: id}
		var resp Empty
		err = c.Call("Docker.PullImage", req, &resp)
		return nil, err
	})
	p.wait()
	return nil
}
