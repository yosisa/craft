package main

import "github.com/yosisa/craft/rpc"

type CmdPull struct {
	Args struct {
		Image string `positional-arg-name:"IMAGE"`
	} `positional-args:"yes" required:"yes"`
}

func (opts *CmdPull) Execute(args []string) error {
	logRPCError(rpc.PullImage(gopts.agents(), opts.Args.Image))
	return nil
}

func init() {
	parser.AddCommand("pull", "Pull a container image", "", &CmdPull{})
}
