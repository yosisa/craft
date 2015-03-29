package mux

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type Handler interface {
	HandleTCP(net.Conn)
}

type HandlerFunc func(net.Conn)

func (f HandlerFunc) HandleTCP(c net.Conn) {
	f(c)
}

type Mux struct {
	h map[byte]Handler
	m sync.RWMutex
}

func (m *Mux) Handle(typ byte, h Handler) {
	m.m.Lock()
	defer m.m.Unlock()
	if m.h == nil {
		m.h = make(map[byte]Handler)
	}
	m.h[typ] = h
}

func (m *Mux) Dispatch(c net.Conn) error {
	typ := make([]byte, 1)
	_, err := c.Read(typ)
	if err != nil {
		return err
	}
	m.m.RLock()
	defer m.m.RUnlock()
	if h := m.h[typ[0]]; h != nil {
		h.HandleTCP(c)
		return nil
	}
	c.Close()
	return fmt.Errorf("Unknown type: %x", typ[0])
}

var DefaultMux = &Mux{}

func Handle(typ byte, h Handler) {
	DefaultMux.Handle(typ, h)
}

func Dispatch(c net.Conn) error {
	return DefaultMux.Dispatch(c)
}

func Dial(network, address string, typ byte) (net.Conn, error) {
	c, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	return newClient(c, typ)
}

func DialTimeout(network, address string, typ byte, timeout time.Duration) (net.Conn, error) {
	c, err := net.DialTimeout(network, address, timeout)
	if err != nil {
		return nil, err
	}
	return newClient(c, typ)
}

func newClient(c net.Conn, typ byte) (net.Conn, error) {
	_, err := c.Write([]byte{typ})
	if err != nil {
		return nil, err
	}
	return c, nil
}
