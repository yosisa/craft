package main

import (
	"errors"
	"math/rand"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/jessevdk/go-flags"
	"github.com/yosisa/craft/config"
	"github.com/yosisa/craft/docker"
	"github.com/yosisa/craft/filter"
	"github.com/yosisa/craft/rpc"
)

type GlobalOptions struct {
	Config string `short:"c" long:"config" description:"Configuration file"`
	Agents string `long:"agents" description:"Comma separated agent list" env:"CRAFT_AGENTS"`
	Filter string `short:"F" long:"filter" description:"Filter target agents" env:"CRAFT_FILTER"`
	conf   *config.Config
}

func (opts *GlobalOptions) ParseConfig() *config.Config {
	var err error
	if opts.conf, err = config.Parse(opts.Config); err != nil {
		log.WithField("error", err).Fatal("Could not parse config file")
	}
	if opts.Agents != "" {
		opts.conf.Agents = strings.Split(opts.Agents, ",")
	}
	return opts.conf
}

func (opts *GlobalOptions) agents() []string {
	if opts.conf == nil {
		opts.ParseConfig()
	}
	v, err := filterAgents(opts.conf.Agents, opts.Filter)
	if err != nil {
		log.WithField("error", err).Fatal("Failed to filter target agents")
	}
	return v
}

func filterAgents(agents []string, s string) ([]string, error) {
	if s == "" {
		return agents, nil
	}
	expr, err := filter.Parse(s)
	if err != nil {
		return nil, err
	}
	caps := gatherCapabilities(agents)
	caps.Filter(func(cap *rpc.Capability) bool {
		return expr.Eval(cap)
	})
	var out []string
	for agent := range caps {
		out = append(out, agent)
	}
	return out, nil
}

var (
	gopts  GlobalOptions
	parser = flags.NewParser(&gopts, flags.Default|flags.IgnoreUnknown)
)

func main() {
	if os.Getenv("GOMAXPROCS") == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	if _, err := parser.Parse(); err != nil {
		os.Exit(1)
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

func (c Capabilities) Agents() []string {
	out := make([]string, 0, len(c))
	for agent := range c {
		out = append(out, agent)
	}
	return out
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
				log.WithFields(log.Fields{"error": err, "agent": agent}).Error("Failed to connect agent")
				return
			}
			defer c.Close()

			var cap rpc.Capability
			if err = c.Call("Craft.Capability", rpc.Empty{}, &cap); err != nil {
				log.WithFields(log.Fields{"error": err, "method": "Craft.Capability"}).Error("RPC failed")
				return
			}
			if !cap.Available {
				log.WithField("agent", agent).Info("Temporary unavailable")
				return
			}
			caps[agent] = &cap
		}(agent)
	}
	wg.Wait()
	return caps
}

func findBestAgent(m *docker.Manifest, caps Capabilities) string {
	if m.Replace == "" {
		// Check availability of name
		caps.Filter(func(cap *rpc.Capability) bool {
			return !stringSlice(cap.AllNames).Contains(m.Name)
		})
	} else {
		// Check existence of a container to be replaced
		caps.Filter(func(cap *rpc.Capability) bool {
			return stringSlice(cap.AllNames).Contains(m.Replace)
		})
		if m.Name == m.Replace {
			// Already satisfied but prefer non-running container
			caps2 := caps.Copy()
			caps.Filter(func(cap *rpc.Capability) bool {
				return !stringSlice(cap.UsedNames).Contains(m.Name)
			})
			if len(caps) == 0 {
				caps = caps2
			}
		} else {
			caps.Filter(func(cap *rpc.Capability) bool {
				return !stringSlice(cap.AllNames).Contains(m.Name)
			})
		}
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

	// Check existence of volume containers
	if len(m.VolumesFrom) > 0 {
		for _, volume := range m.VolumesFrom {
			caps.Filter(func(cap *rpc.Capability) bool {
				return stringSlice(cap.AllNames).Contains(volume)
			})
		}
	}

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

	// Label restriction
	if len(m.Restrict.Labels) > 0 {
		for key, value := range m.Restrict.Labels {
			caps.Filter(func(cap *rpc.Capability) bool {
				return cap.Labels[key] == value
			})
		}
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
			fields := log.Fields{"error": strings.TrimSpace(err.Error()), "agent": addr}
			log.WithFields(fields).Error("RPC error")
		})
	} else {
		log.WithField("error", err).Error("RPC error")
	}
}

func sortedKeys(m map[string]interface{}) []string {
	var out []string
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
