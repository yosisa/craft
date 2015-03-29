package rpc

import (
	"encoding/binary"
	"log"
	"net"
	"net/rpc"
	"strings"

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

func StartContainer(addrs []string, container string) error {
	CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		err := c.Call("Docker.StartContainer", container, &Empty{})
		if err != nil && strings.Contains(err.Error(), "No such container") {
			err = nil
		}
		return nil, err
	})
	return nil
}

func StopContainer(addrs []string, container string, timeout uint) error {
	CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		req := StopContainerRequest{ID: container, Timeout: timeout}
		err := c.Call("Docker.StopContainer", req, &Empty{})
		if err != nil && strings.Contains(err.Error(), "No such container") {
			err = nil
		}
		return nil, err
	})
	return nil
}

func RemoveContainer(addrs []string, container string, force bool) error {
	CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		req := RemoveContainerRequest{ID: container, Force: force}
		err := c.Call("Docker.RemoveContainer", req, &Empty{})
		if err != nil && strings.HasPrefix(err.Error(), "No such container:") {
			err = nil
		}
		return nil, err
	})
	return nil
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
