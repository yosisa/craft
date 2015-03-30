package main

import "github.com/yosisa/craft/rpc"

type CmdRm struct {
	Force bool `short:"f" long:"force" description:"Force remove running container"`
	Args  struct {
		Container string `positional-arg-name:"CONTAINER"`
	} `positional-args:"yes" required:"yes"`
}

func (opts *CmdRm) Execute(args []string) error {
	conf := gopts.ParseConfig()
	logRPCError(rpc.RemoveContainer(conf.Agents, opts.Args.Container, opts.Force))
	return nil
}

func init() {
	parser.AddCommand("rm", "Remove a container", "", &CmdRm{})
}
