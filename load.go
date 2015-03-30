package main

import (
	"io"
	"os"

	"github.com/yosisa/craft/rpc"
)

type CmdLoad struct {
	Input    string `short:"i" long:"input" description:"Input file" default:"-"`
	Pipeline bool   `long:"pipeline" description:"Send an image using pipeline"`
}

func (opts *CmdLoad) Execute(args []string) (err error) {
	conf := gopts.ParseConfig()
	var r io.Reader
	if path := opts.Input; path == "-" {
		r = os.Stdin
	} else {
		if r, err = os.Open(path); err != nil {
			return
		}
	}
	logRPCError(rpc.LoadImage(conf.Agents, r, opts.Pipeline))
	return
}

func init() {
	parser.AddCommand("load", "Load a container image from tarball", "", &CmdLoad{})
}
