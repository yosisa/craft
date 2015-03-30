package main

import "github.com/yosisa/craft/rpc"

type CmdLogs struct {
	Follow bool   `short:"f" long:"follow" description:"Continuous tailing logs"`
	Tail   string `long:"tail" description:"Number of recent logs" default:"all"`
	Args   struct {
		Container string `positional-arg-name:"CONTAINER"`
	} `positional-args:"yes" required:"yes"`
}

func (opts *CmdLogs) Execute(args []string) error {
	conf := gopts.ParseConfig()
	logRPCError(rpc.Logs(conf.Agents, opts.Args.Container, opts.Follow, opts.Tail))
	return nil
}

func init() {
	parser.AddCommand("logs", "Show logs of a container", "", &CmdLogs{})
}
