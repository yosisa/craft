package rpc

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/rpc"
	"strings"
	"sync"
	"time"

	"github.com/yosisa/craft/config"
	"github.com/yosisa/craft/docker"
	"github.com/yosisa/craft/mux"
)

const dialTimeout = 5 * time.Second

var (
	agentName    string
	ipAddrs      []string
	streamWriter = make(chan io.WriteCloser)
)

const (
	chanRPC byte = iota
	chanNewStream
)

type Empty struct{}

type Capability struct {
	Available  bool
	Agent      string
	IPAddrs    []string
	AllNames   []string
	UsedNames  []string
	UsedPorts  []int64
	Containers map[string]*docker.ContainerInfo
}

type SubmitRequest struct {
	Manifest *docker.Manifest
	ExLinks  []*ExLink
}

type ExLink struct {
	Name    string
	Exposed string
	Addr    string
	Port    int
}

func (l *ExLink) Env() map[string]string {
	name := strings.ToUpper(l.Name)
	parts := strings.Split(l.Exposed, "/")
	port, proto := parts[0], parts[1]
	prefix := fmt.Sprintf("%s_PORT_%s_%s", name, port, strings.ToUpper(proto))
	v := map[string]string{
		name + "_PORT":    fmt.Sprintf("%s://%s:%d", proto, l.Addr, l.Port),
		prefix + "_ADDR":  l.Addr,
		prefix + "_PORT":  fmt.Sprintf("%d", l.Port),
		prefix + "_PROTO": proto,
	}
	v[prefix] = v[name+"_PORT"]
	return v
}

type SubmitResponse struct {
	Agent string
}

type Craft struct {
	c  *docker.Client
	lc chan struct{}
}

func (c *Craft) Capability(req Empty, resp *Capability) error {
	if !c.lockNoWait() {
		return nil
	}
	defer c.unlock()

	ui, err := c.c.Usage()
	if err != nil {
		return err
	}
	resp.Available = true
	resp.Agent = agentName
	resp.IPAddrs = ipAddrs
	resp.AllNames = ui.AllNames
	resp.UsedNames = ui.UsedNames
	resp.UsedPorts = ui.UsedPorts
	resp.Containers = ui.Containers
	return nil
}

func (c *Craft) Submit(req SubmitRequest, resp *SubmitResponse) error {
	c.lock()
	defer c.unlock()
	w := <-streamWriter
	defer w.Close()

	for _, exl := range req.ExLinks {
		req.Manifest.MergeEnv(exl.Env())
	}
	if err := c.c.Run(req.Manifest, w); err != nil {
		return err
	}
	resp.Agent = agentName
	return nil
}

func (c *Craft) lock() {
	<-c.lc
}

func (c *Craft) lockNoWait() bool {
	select {
	case <-c.lc:
		return true
	default:
		return false
	}
}

func (c *Craft) unlock() {
	c.lc <- struct{}{}
}

func ListenAndServe(c *config.Config) error {
	agentName = c.AgentName
	ips, err := ListIPAddrs()
	if err != nil {
		return err
	}
	ipAddrs = ips

	client, err := docker.NewClient(c.Docker)
	if err != nil {
		return err
	}
	craft := &Craft{
		c:  client,
		lc: make(chan struct{}, 1),
	}
	craft.lc <- struct{}{}
	rpc.Register(craft)

	d, err := NewDocker(c.Docker)
	if err != nil {
		return err
	}
	rpc.Register(d)

	mux.Handle(chanRPC, mux.HandlerFunc(func(c net.Conn) {
		rpc.ServeConn(c)
	}))
	mux.Handle(chanNewStream, mux.HandlerFunc(func(c net.Conn) {
		streamWriter <- c
	}))

	ln, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return err
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		go mux.HandleTCP(conn)
	}
	return nil
}

func Dial(network, address string) (*rpc.Client, error) {
	conn, err := mux.DialTimeout(network, address, chanRPC, 5*time.Second)
	if err != nil {
		return nil, err
	}
	return rpc.NewClient(conn), nil
}

func Submit(address string, req SubmitRequest) (*SubmitResponse, error) {
	c, err := Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	p := newProgress()
	go p.show()
	go func() {
		sc, err := mux.DialTimeout("tcp", address, chanNewStream, dialTimeout)
		if err != nil {
			log.Print(err)
			return
		}
		p.add(sc, address)
	}()

	var resp SubmitResponse
	err = c.Call("Craft.Submit", req, &resp)
	p.wait()
	if err != nil {
		return nil, err
	}
	return &resp, err
}

func CallAll(addrs []string, f func(c *rpc.Client, addr string) (interface{}, error)) map[string]interface{} {
	var wg sync.WaitGroup
	wg.Add(len(addrs))
	out := make(map[string]interface{})
	for _, addr := range addrs {
		go func(addr string) {
			defer wg.Done()
			c, err := Dial("tcp", addr)
			if err != nil {
				log.Print(err)
				return
			}
			defer c.Close()

			resp, err := f(c, addr)
			if err != nil {
				log.Print(err)
				return
			}
			out[addr] = resp
		}(addr)
	}
	wg.Wait()
	return out
}
