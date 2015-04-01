package main

import (
	"io"
	"os"

	"github.com/dustin/go-humanize"
	"github.com/yosisa/craft/rpc"
)

type ByteSize uint64

func (n *ByteSize) UnmarshalFlag(value string) error {
	v, err := humanize.ParseBytes(value)
	if err != nil {
		return err
	}
	*n = ByteSize(v)
	return nil
}

type CmdLoad struct {
	Input    string   `short:"i" long:"input" description:"Input file" default:"-"`
	Pipeline bool     `long:"pipeline" description:"Send an image using pipeline"`
	Compress bool     `long:"compress" description:"Compress stream using LZ4"`
	BWLimit  ByteSize `long:"bwlimit" description:"Limit bandwidth"`
}

func (opts *CmdLoad) Execute(args []string) (err error) {
	var r io.Reader
	if path := opts.Input; path == "-" {
		r = os.Stdin
	} else {
		if r, err = os.Open(path); err != nil {
			return
		}
	}
	if opts.Pipeline {
		logRPCError(rpc.LoadImageUsingPipeline(gopts.agents(), r, opts.Compress, uint64(opts.BWLimit)))
	} else {
		logRPCError(rpc.LoadImage(gopts.agents(), r, opts.Compress, uint64(opts.BWLimit)))
	}
	return
}

func init() {
	parser.AddCommand("load", "Load a container image from tarball", "", &CmdLoad{})
}
