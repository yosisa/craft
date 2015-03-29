package rpc

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

const allocTimeout = 10 * time.Second

type StreamConn struct {
	p map[uint32]chan net.Conn
	m sync.RWMutex
}

func NewStreamConn() *StreamConn {
	return &StreamConn{
		p: make(map[uint32]chan net.Conn),
	}
}

type AllocResponse struct {
	ID uint32
}

func (s *StreamConn) Alloc(req Empty, resp *AllocResponse) error {
	s.m.Lock()
	defer s.m.Unlock()
	for {
		id := rand.Uint32()
		if _, ok := s.p[id]; !ok {
			s.p[id] = make(chan net.Conn, 1)
			resp.ID = id
			time.AfterFunc(allocTimeout, func() { s.release(id) })
			return nil
		}
	}
}

func (s *StreamConn) put(conn net.Conn) error {
	var id uint32
	if err := binary.Read(conn, binary.BigEndian, &id); err != nil {
		return err
	}
	c, err := s.getChan(id)
	if err != nil {
		return err
	}
	c <- conn
	return nil
}

func (s *StreamConn) get(id uint32) (net.Conn, error) {
	c, err := s.getChan(id)
	if err != nil {
		return nil, err
	}
	conn, ok := <-c
	if !ok {
		return nil, fmt.Errorf("Timeout: acquiring stream connection: %d", id)
	}
	s.release(id)
	return conn, nil
}

func (s *StreamConn) getChan(id uint32) (chan net.Conn, error) {
	s.m.RLock()
	defer s.m.RUnlock()
	c, ok := s.p[id]
	if !ok {
		return nil, fmt.Errorf("Invalid stream id: %d", id)
	}
	return c, nil
}

func (s *StreamConn) release(id uint32) {
	s.m.Lock()
	defer s.m.Unlock()
	if c, ok := s.p[id]; ok {
		delete(s.p, id)
		select {
		case conn := <-c:
			conn.Close()
		default:
		}
		close(c)
	}
}

var streamConn = NewStreamConn()

func init() {
	rand.Seed(time.Now().UnixNano())
}
