package mux

import (
	"log"
	"net"
	"sync"
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

func (m *Mux) HandleTCP(c net.Conn) {
	typ := make([]byte, 1)
	_, err := c.Read(typ)
	if err != nil {
		log.Print(err)
	}
	m.m.RLock()
	defer m.m.RUnlock()
	if h := m.h[typ[0]]; h != nil {
		h.HandleTCP(c)
		return
	}
	log.Printf("Unknown type: %x", typ[0])
	c.Close()
}

type typeWriter struct {
	net.Conn
	typ     byte
	typSent bool
}

func (w *typeWriter) Write(b []byte) (n int, err error) {
	if !w.typSent {
		n, err = w.Conn.Write([]byte{w.typ})
		w.typSent = true
		if err != nil {
			return
		}
	}
	nn, err := w.Conn.Write(b)
	return nn + n, err
}

var DefaultMux = &Mux{}

func Handle(typ byte, h Handler) {
	DefaultMux.Handle(typ, h)
}

func HandleTCP(c net.Conn) {
	DefaultMux.HandleTCP(c)
}

func NewClient(c net.Conn, typ byte) net.Conn {
	return &typeWriter{Conn: c, typ: typ}
}
