package rpc

import (
	"encoding/binary"
	"net"
	"net/rpc"
	"strings"

	log "github.com/Sirupsen/logrus"
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
	_, err := CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		err := c.Call("Docker.StartContainer", container, &Empty{})
		if err == nil {
			fields := log.Fields{"agent": addr, "container": container}
			log.WithFields(fields).Info("Container started")
		} else {
			err = safeError(err)
		}
		return nil, err
	})
	return err
}

func StopContainer(addrs []string, container string, timeout uint) error {
	_, err := CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		req := StopContainerRequest{ID: container, Timeout: timeout}
		err := c.Call("Docker.StopContainer", req, &Empty{})
		if err == nil {
			fields := log.Fields{"agent": addr, "container": container}
			log.WithFields(fields).Info("Container stopped")
		} else {
			err = safeError(err)
		}
		return nil, err
	})
	return err
}

func RemoveContainer(addrs []string, container string, force bool) error {
	_, err := CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		req := RemoveContainerRequest{ID: container, Force: force}
		err := c.Call("Docker.RemoveContainer", req, &Empty{})
		if err == nil {
			fields := log.Fields{"agent": addr, "container": container}
			log.WithFields(fields).Info("Container removed")
		} else {
			err = safeError(err)
		}
		return nil, err
	})
	return err
}

func PullImage(addrs []string, image string) error {
	p := newProgress()
	go p.show()
	_, err := CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		id, sc, err := AllocStream(c, addr)
		if err != nil {
			return nil, err
		}
		p.add(sc, addr)
		req := PullImageRequest{Image: image, StreamID: id}
		var resp Empty
		err = c.Call("Docker.PullImage", req, &resp)
		return nil, err
	})
	p.wait()
	return err
}

func safeError(err error) error {
	if err != nil && strings.Contains(err.Error(), "No such container") {
		return nil
	}
	return err
}
