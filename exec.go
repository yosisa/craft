package main

import (
	"errors"

	"github.com/yosisa/craft/rpc"
)

type CmdExec struct {
	Interactive bool `short:"i" long:"interactive" description:"Interactive mode"`
	Tty         bool `short:"t" long:"tty" description:"Allocate a pseudo-TTY"`
	Args        struct {
		Container string   `positional-arg-name:"CONTAINER"`
		Command   string   `positional-arg-name:"COMMAND"`
		Args      []string `positional-arg-name:"ARG"`
	} `positional-args:"yes" required:"yes"`
}

func (opts *CmdExec) Execute(args []string) error {
	cmd := []string{opts.Args.Command}
	cmd = append(cmd, opts.Args.Args...)
	agents := gopts.agents()
	if opts.Interactive && len(agents) > 1 {
		caps := gatherCapabilities(agents)
		caps.Filter(func(cap *rpc.Capability) bool {
			return stringSlice(cap.UsedNames).Contains(opts.Args.Container)
		})
		if len(caps) > 1 {
			return errors.New("Unique agent required for interactive mode")
		}
		agents = caps.Agents()
	}
	logRPCError(rpc.Exec(agents, opts.Args.Container, cmd, opts.Interactive, opts.Tty))
	return nil
}

func init() {
	parser.AddCommand("exec", "Exec command in a container", "", &CmdExec{})
}
