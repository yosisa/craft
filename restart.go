package main

import "github.com/yosisa/craft/rpc"

type CmdRestart struct {
	Timeout uint `short:"t" long:"time" description:"Wait for the container to stop in seconds" default:"10"`
	Args    struct {
		Container string `positional-arg-name:"CONTAINER"`
	} `positional-args:"yes" required:"yes"`
}

func (opts *CmdRestart) Execute(args []string) error {
	logRPCError(rpc.RestartContainer(gopts.agents(), opts.Args.Container, opts.Timeout))
	return nil
}

func init() {
	parser.AddCommand("restart", "Restart a container", "", &CmdRestart{})
}
