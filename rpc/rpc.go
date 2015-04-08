package rpc

import (
	"fmt"
	"net"
	"net/rpc"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/yosisa/craft/config"
	"github.com/yosisa/craft/docker"
	"github.com/yosisa/craft/mux"
)

const dialTimeout = 5 * time.Second

var (
	agentName string
	labels    map[string]string
	ipAddrs   []string
)

const (
	chanRPC byte = iota
	chanNewStream
)

type Empty struct{}

type Capability struct {
	Available  bool
	Agent      string
	Labels     map[string]string
	IPAddrs    []string
	AllNames   []string
	UsedNames  []string
	UsedPorts  []int64
	Containers map[string]*docker.ContainerInfo
}

type SubmitRequest struct {
	Manifest *docker.Manifest
	ExLinks  []*ExLink
	StreamID uint32
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
	resp.Labels = labels
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
	w, err := streamConn.get(req.StreamID)
	if err != nil {
		return err
	}
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
	labels = c.Labels
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
	rpc.Register(streamConn)

	mux.Handle(chanRPC, mux.HandlerFunc(func(c net.Conn) {
		rpc.ServeConn(c)
	}))
	mux.Handle(chanNewStream, mux.HandlerFunc(func(c net.Conn) {
		if err := streamConn.put(c); err != nil {
			log.WithField("error", err).Error("Failed to put stream connection")
			c.Close()
		}
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
		go func() {
			if err := mux.Dispatch(conn); err != nil {
				log.WithField("error", err).Error("Failed to dispatch connection")
			}
		}()
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

func Submit(address string, m *docker.Manifest, exlinks []*ExLink) (*SubmitResponse, error) {
	c, err := Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	id, sc, err := AllocStream(c, address)
	if err != nil {
		return nil, err
	}

	p := newProgress()
	go p.show()
	p.add(sc, address)

	req := SubmitRequest{
		Manifest: m,
		ExLinks:  exlinks,
		StreamID: id,
	}
	var resp SubmitResponse
	err = c.Call("Craft.Submit", req, &resp)
	p.wait()
	if err != nil {
		return nil, err
	}
	return &resp, err
}

func CallAll(addrs []string, f func(c *rpc.Client, addr string) (interface{}, error)) (map[string]interface{}, error) {
	var wg sync.WaitGroup
	wg.Add(len(addrs))
	out := make(map[string]interface{})
	errs := make(Error)
	for _, addr := range addrs {
		go func(addr string) {
			defer wg.Done()
			c, err := Dial("tcp", addr)
			if err != nil {
				log.WithFields(log.Fields{"error": err, "agent": addr}).Error("Failed to connect agent")
				return
			}
			defer c.Close()

			resp, err := f(c, addr)
			if err != nil {
				errs[addr] = err
				return
			}
			out[addr] = resp
		}(addr)
	}
	wg.Wait()
	if len(errs) == 0 {
		return out, nil
	}
	return out, errs
}

type Error map[string]error

func (e Error) Error() string {
	var s []string
	for addr, err := range e {
		s = append(s, fmt.Sprintf("[%s] %s", addr, strings.TrimSpace(err.Error())))
	}
	return strings.Join(s, ", ")
}

func (e Error) Each(f func(string, error)) {
	for addr, err := range e {
		f(addr, err)
	}
}
