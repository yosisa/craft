package main

import "github.com/yosisa/craft/rpc"

type CmdPull struct {
	Args struct {
		Image string `positional-arg-name:"IMAGE"`
	} `positional-args:"yes" required:"yes"`
}

func (opts *CmdPull) Execute(args []string) error {
	conf := gopts.ParseConfig()
	logRPCError(rpc.PullImage(conf.Agents, opts.Args.Image))
	return nil
}

func init() {
	parser.AddCommand("pull", "Pull a container image", "", &CmdPull{})
}
