package main

import "github.com/yosisa/craft/rpc"

type CmdStart struct {
	Args struct {
		Container string `positional-arg-name:"CONTAINER"`
	} `positional-args:"yes" required:"yes"`
}

func (opts *CmdStart) Execute(args []string) error {
	logRPCError(rpc.StartContainer(gopts.agents(), opts.Args.Container))
	return nil
}

func init() {
	parser.AddCommand("start", "Start a container", "", &CmdStart{})
}
