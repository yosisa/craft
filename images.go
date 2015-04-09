package main

import (
	"fmt"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/yosisa/craft/docker"
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
		var tw tableWriter
		tw.Append("REPOSITORY", "TAG", "IMAGE ID", "CREATED", "VIRTUAL SIZE")
		for _, i := range imgs {
			var repo, tag string
			if i.RepoTags[0] == "<none>:<none>" {
				repo, tag = "<none>", "<none>"
			} else {
				repo, tag = docker.SplitImageTag(i.RepoTags[0])
			}
			tw.Append(repo, tag, i.ID[:12], humanize.Time(time.Unix(i.Created, 0)),
				humanize.Bytes(uint64(i.VirtualSize)))
		}
		tw.Write(os.Stdout, "  ")
		fmt.Println()
	}
	return nil
}

func init() {
	parser.AddCommand("images", "List images", "", &CmdImages{})
}
