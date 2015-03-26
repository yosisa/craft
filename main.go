package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/yosisa/craft/config"
	"github.com/yosisa/craft/docker"
	"github.com/yosisa/craft/rpc"
	"gopkg.in/alecthomas/kingpin.v1"
)

var (
	configPath     = kingpin.Flag("config", "").Short('c').String()
	agent          = kingpin.Command("agent", "")
	usage          = kingpin.Command("usage", "")
	submit         = kingpin.Command("submit", "")
	submitManifest = submit.Arg("file", "").Required().String()
)

func main() {
	kingpin.Version("0.1.0")
	cmd := kingpin.Parse()
	conf, err := config.Parse(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	switch cmd {
	case agent.FullCommand():
		if err := rpc.ListenAndServe(conf); err != nil {
			log.Fatal(err)
		}
	case usage.FullCommand():
		c, err := docker.NewClient(conf.Docker)
		if err != nil {
			log.Fatal(err)
		}
		ui, err := c.Usage()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%+v\n", ui)
	case submit.FullCommand():
		m, err := docker.ParseManifest(*submitManifest)
		if err != nil {
			log.Fatal(err)
		}

		caps := gatherCapabilities(conf.Agents)
		agent := findBestAgent(m, caps.Copy())
		if agent == "" {
			log.Fatal(errors.New("No available agents"))
		}
		exlinks, err := resolveExLinks(m, caps)
		if err != nil {
			log.Fatal(err)
		}

		req := rpc.SubmitRequest{
			Manifest: m,
			ExLinks:  exlinks,
		}
		resp, err := rpc.Submit(agent, req)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Container %s runs on %s", m.Name, resp.Agent)
	}
}

type Capabilities map[string]*rpc.Capability

func (c Capabilities) Filter(f func(*rpc.Capability) bool) {
	for agent, cap := range c {
		if !f(cap) {
			delete(c, agent)
		}
	}
}

func (c Capabilities) Copy() Capabilities {
	caps := make(Capabilities, len(c))
	for agent, cap := range c {
		caps[agent] = cap
	}
	return caps
}

func gatherCapabilities(agents []string) Capabilities {
	var wg sync.WaitGroup
	wg.Add(len(agents))
	caps := make(Capabilities)
	for _, agent := range agents {
		func(agent string) {
			defer wg.Done()
			c, err := rpc.Dial("tcp", agent)
			if err != nil {
				log.Print(err)
				return
			}
			defer c.Close()

			var cap rpc.Capability
			if err = c.Call("Craft.Capability", rpc.Empty{}, &cap); err != nil {
				log.Print(err)
				return
			}
			if !cap.Available {
				log.Printf("%s temporary unavailable", agent)
				return
			}
			caps[agent] = &cap
		}(agent)
	}
	wg.Wait()
	return caps
}

func findBestAgent(m *docker.Manifest, caps Capabilities) string {
	// Check existence of a container to be replaced
	if m.Replace != "" {
		caps.Filter(func(cap *rpc.Capability) bool {
			return stringSlice(cap.AllNames).Contains(m.Replace)
		})
	}

	// Check availability of name
	caps2 := caps.Copy()
	caps.Filter(func(cap *rpc.Capability) bool {
		return !stringSlice(cap.UsedNames).Contains(m.Name)
	})
	if len(caps) == 0 && m.Name == m.Replace {
		caps = caps2
	}

	// Check availability of ports
	caps.Filter(func(cap *rpc.Capability) bool {
		for _, p := range m.Ports {
			becomeAvailable, ok := cap.Containers[m.Replace]
			if ok && portSpecSlice(becomeAvailable.Ports).Contains(p.HostPort) {
				continue
			}
			if int64Slice(cap.UsedPorts).Contains(p.HostPort) {
				return false
			}
		}
		return true
	})

	// Check existence of containers to be linked
	caps.Filter(func(cap *rpc.Capability) bool {
		for _, link := range m.Links {
			if !stringSlice(cap.UsedNames).Contains(link.Name) {
				return false
			}
		}
		return true
	})

	// Check existence of a network container
	if strings.HasPrefix(m.NetworkMode, "container:") {
		name := m.NetworkMode[10:]
		caps.Filter(func(cap *rpc.Capability) bool {
			return stringSlice(cap.UsedNames).Contains(name)
		})
	}

	// Agent name restriction
	if m.Restrict.Agent != "" {
		re, _ := regexp.Compile(m.Restrict.Agent)
		caps.Filter(func(cap *rpc.Capability) bool {
			return re.MatchString(cap.Agent)
		})
	}

	// Choice agent that has least active containers
	running := 1024 * 1024 * 1024 // it's large enough
	var agent string
	for addr, cap := range caps {
		if n := len(cap.UsedNames); n < running {
			running = n
			agent = addr
		}
	}
	return agent
}

func resolveExLinks(m *docker.Manifest, caps Capabilities) ([]*rpc.ExLink, error) {
	var out []*rpc.ExLink
	for _, l := range m.ExLinks {
		caps2 := caps.Copy()
		caps2.Filter(func(cap *rpc.Capability) bool {
			return stringSlice(cap.UsedNames).Contains(l.Name) &&
				len(cap.IPAddrs) > 0
		})
		if len(caps2) == 0 {
			return nil, errors.New("No linkable containers")
		}
		_, cap := choice(caps2)
		ci := cap.Containers[l.Name]
		for _, port := range ci.Ports {
			addr := port.HostIP
			if addr == "0.0.0.0" {
				addr = cap.IPAddrs[0]
			}
			out = append(out, &rpc.ExLink{
				Name:    l.Alias,
				Exposed: string(port.Exposed),
				Addr:    addr,
				Port:    int(port.HostPort),
			})
		}
	}
	return out, nil
}

func choice(caps Capabilities) (string, *rpc.Capability) {
	var keys []string
	for k := range caps {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	n := rand.Intn(len(keys))
	return keys[n], caps[keys[n]]
}

type stringSlice []string

func (ss stringSlice) Contains(s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

type int64Slice []int64

func (is int64Slice) Contains(n int64) bool {
	for _, v := range is {
		if v == n {
			return true
		}
	}
	return false
}

type portSpecSlice []*docker.PortSpec

func (ps portSpecSlice) Contains(n int64) bool {
	for _, v := range ps {
		if v.HostPort == n {
			return true
		}
	}
	return false
}
