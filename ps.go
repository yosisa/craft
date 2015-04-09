package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/yosisa/craft/docker"
	"github.com/yosisa/craft/rpc"
)

type CmdPs struct {
	All  bool `short:"a" long:"all" description:"Show all containers"`
	Full bool `long:"full" description:"Show full command"`
	Args struct {
		Containers []string `positional-arg-name:"CONTAINER"`
	} `positional-args:"yes"`
}

func (opts *CmdPs) Execute(args []string) error {
	containers, err := rpc.ListContainers(gopts.agents(), opts.All)
	logRPCError(err)
	for _, agent := range sortedKeys(containers) {
		fmt.Printf("[%s]\n", agent)
		cons := containers[agent].(*rpc.ListContainersResponse).FilterByNames(opts.Args.Containers)
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
		if nc > 20 && !opts.Full {
			nc = 20
		}
		s := "  %-15s%-" + strconv.Itoa(nn+3) + "s%-" + strconv.Itoa(ni+3) + "s%-" +
			strconv.Itoa(nc+3) + "s%-" + strconv.Itoa(nt+3) + "s%-" + strconv.Itoa(ns+3) + "s%s\n"
		fmt.Printf(s, "CONTAINER ID", "NAME", "IMAGE", "COMMAND", "CREATED", "STATUS", "PORTS")
		for _, c := range cons {
			cmd := c.Command
			if len(cmd) > 20 && !opts.Full {
				cmd = cmd[:20]
			}
			fmt.Printf(s, c.ID[:12], docker.CanonicalName(c.Names), c.Image, cmd,
				humanize.Time(time.Unix(c.Created, 0)), c.Status, docker.FormatPorts(c.Ports))
		}
		fmt.Println()
	}
	return nil
}

func init() {
	parser.AddCommand("ps", "List containers", "", &CmdPs{})
}
