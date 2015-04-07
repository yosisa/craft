package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/yosisa/craft/rpc"
)

type CmdImages struct{}

func (opts *CmdImages) Execute(args []string) error {
	images, err := rpc.ListImages(gopts.agents())
	logRPCError(err)
	for _, agent := range sortedKeys(images) {
		fmt.Printf("[%s]\n", agent)
		imgs := images[agent].(*rpc.ListImagesResponse).Images
		if len(imgs) == 0 {
			fmt.Println()
			continue
		}
		nr, nt, nc, ns := 10, 3, 7, 12
		for _, i := range imgs {
			repoTag := strings.SplitN(i.RepoTags[0], ":", 2)
			if n := len(repoTag[0]); n > nr {
				nr = n
			}
			if n := len(repoTag[1]); n > nt {
				nt = n
			}
			if n := len(humanize.Time(time.Unix(i.Created, 0))); n > nc {
				nc = n
			}
			if n := len(humanize.Bytes(uint64(i.VirtualSize))); n > ns {
				ns = n
			}
		}
		s := "  " + dw(nr) + dw(nt) + dw(12) + dw(nc) + dw(ns) + "\n"
		fmt.Printf(s, "REPOSITORY", "TAG", "IMAGE ID", "CREATED", "VIRTUAL SIZE")
		for _, i := range imgs {
			repoTag := strings.SplitN(i.RepoTags[0], ":", 2)
			fmt.Printf(s, repoTag[0], repoTag[1], i.ID[:12], humanize.Time(time.Unix(i.Created, 0)),
				humanize.Bytes(uint64(i.VirtualSize)))
		}
		fmt.Println()
	}
	return nil
}

func dw(n int) string {
	return "%-" + strconv.Itoa(n+3) + "s"
}

func init() {
	parser.AddCommand("images", "List images", "", &CmdImages{})
}
