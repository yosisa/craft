package main

import "github.com/yosisa/craft/rpc"

type CmdRmi struct {
	Args struct {
		Image string `positional-arg-name:"IMAGE"`
	} `positional-args:"yes" required:"yes"`
}

func (opts *CmdRmi) Execute(args []string) error {
	logRPCError(rpc.RemoveImage(gopts.agents(), opts.Args.Image))
	return nil
}

func init() {
	parser.AddCommand("rmi", "Remove an image", "", &CmdRmi{})
}
