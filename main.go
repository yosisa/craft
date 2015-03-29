package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	nrpc "net/rpc"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/dustin/go-humanize"
	"github.com/yosisa/craft/config"
	"github.com/yosisa/craft/docker"
	"github.com/yosisa/craft/rpc"
)

var usage = `Docker provisioning tool

Usage:
  craft [-c FILE] agent
  craft [-c FILE] usage
  craft [-c FILE] submit MANIFEST
  craft [-c FILE] ps [-a] [--full]
  craft [-c FILE] rm [-f] CONTAINER
  craft [-c FILE] pull IMAGE
  craft [-c FILE] start CONTAINER
  craft [-c FILE] stop [-t TIMEOUT] CONTAINER
  craft -h | --help
  craft --version

Options:
  -h --help                  Show this screen.
  --version                  Show version.
  -c FILE --config=FILE      Configuration file.
  -a --all                   List all containers.
  --full                     Show full command.
  -f                         Force remove.
  -t TIMEOUT --time=TIMEOUT  Wait for the container to stop in seconds [default: 10].
`

func main() {
	args, err := docopt.Parse(usage, nil, true, "craft 0.1.0", false)
	if err != nil {
		log.Fatal(err)
	}

	var configPath string
	if v, ok := args["--config"].(string); ok {
		configPath = v
	}
	conf, err := config.Parse(configPath)
	if err != nil {
		log.Fatal(err)
	}

	switch {
	case args["agent"]:
		if err := rpc.ListenAndServe(conf); err != nil {
			log.Fatal(err)
		}
	case args["usage"]:
		c, err := docker.NewClient(conf.Docker)
		if err != nil {
			log.Fatal(err)
		}
		ui, err := c.Usage()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%+v\n", ui)
	case args["submit"]:
		m, err := docker.ParseManifest(args["MANIFEST"].(string))
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

		resp, err := rpc.Submit(agent, m, exlinks)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Container %s runs on %s", m.Name, resp.Agent)
	case args["ps"]:
		containers, err := rpc.CallAll(conf.Agents, func(c *nrpc.Client, addr string) (interface{}, error) {
			req := rpc.ListContainersRequest{All: args["--all"].(bool)}
			var resp rpc.ListContainersResponse
			err := c.Call("Docker.ListContainers", req, &resp)
			return &resp, err
		})
		logRPCError(err)
		for agent, resp := range containers {
			fmt.Printf("[%s]\n", agent)
			cons := resp.(*rpc.ListContainersResponse).Containers
			if len(cons) == 0 {
				fmt.Println()
				continue
			}
			var nn, ni, nc, nt, ns int
			for _, c := range cons {
				if n := len(docker.CanonicalName(c.Names)); n > nn {
					nn = n
				}
				if n := len(c.Image); n > ni {
					ni = n
				}
				if n := len(c.Command); n > nc {
					nc = n
				}
				if n := len(humanize.Time(time.Unix(c.Created, 0))); n > nt {
					nt = n
				}
				if n := len(c.Status); n > ns {
					ns = n
				}
			}
			if nc > 20 && !args["--full"].(bool) {
				nc = 20
			}
			s := "  %-15s%-" + strconv.Itoa(nn+3) + "s%-" + strconv.Itoa(ni+3) + "s%-" +
				strconv.Itoa(nc+3) + "s%-" + strconv.Itoa(nt+3) + "s%-" + strconv.Itoa(ns+3) + "s%s\n"
			fmt.Printf(s, "CONTAINER ID", "NAME", "IMAGE", "COMMAND", "CREATED", "STATUS", "PORTS")
			for _, c := range cons {
				cmd := c.Command
				if len(cmd) > 20 && !args["--full"].(bool) {
					cmd = cmd[:20]
				}
				fmt.Printf(s, c.ID[:12], docker.CanonicalName(c.Names), c.Image, cmd,
					humanize.Time(time.Unix(c.Created, 0)), c.Status, docker.FormatPorts(c.Ports))
			}
			fmt.Println()
		}
	case args["rm"]:
		err := rpc.RemoveContainer(conf.Agents, args["CONTAINER"].(string), args["-f"].(bool))
		logRPCError(err)
	case args["start"]:
		err := rpc.StartContainer(conf.Agents, args["CONTAINER"].(string))
		logRPCError(err)
	case args["stop"]:
		timeout, err := strconv.Atoi(args["--time"].(string))
		if err != nil {
			log.Fatal(err)
		}
		err = rpc.StopContainer(conf.Agents, args["CONTAINER"].(string), uint(timeout))
		logRPCError(err)
	case args["pull"]:
		err := rpc.PullImage(conf.Agents, args["IMAGE"].(string))
		logRPCError(err)
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
		re := regexp.MustCompile(m.Restrict.Agent)
		caps.Filter(func(cap *rpc.Capability) bool {
			return re.MatchString(cap.Agent)
		})
	}

	// Conflicts restriction
	for _, conflict := range m.Restrict.Conflicts {
		re := regexp.MustCompile(conflict)
		caps.Filter(func(cap *rpc.Capability) bool {
			return !stringSlice(cap.UsedNames).Match(re)
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

func (ss stringSlice) Match(re *regexp.Regexp) bool {
	for _, v := range ss {
		if re.MatchString(v) {
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

func logRPCError(err error) {
	if err == nil {
		return
	}
	if re, ok := err.(rpc.Error); ok {
		re.Each(func(addr string, err error) {
			log.Printf("[%s] %s", addr, strings.TrimSpace(err.Error()))
		})
	} else {
		log.Print(err)
	}
}
