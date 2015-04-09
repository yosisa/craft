package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/yosisa/craft/docker"
	"github.com/yosisa/craft/rpc"
)

type CmdPs struct {
	All  bool `short:"a" long:"all" description:"Show all containers"`
	Full bool `long:"full" description:"Show full command"`
	Args struct {
		Containers []string `positional-arg-name:"CONTAINER"`
	} `positional-args:"yes"`
}

func (opts *CmdPs) Execute(args []string) error {
	containers, err := rpc.ListContainers(gopts.agents(), opts.All)
	logRPCError(err)
	for _, agent := range sortedKeys(containers) {
		fmt.Printf("[%s]\n", agent)
		cons := containers[agent].(*rpc.ListContainersResponse).FilterByNames(opts.Args.Containers)
		if len(cons) == 0 {
			fmt.Println()
			continue
		}
		var tw tableWriter
		tw.Append("CONTAINER ID", "NAME", "IMAGE", "COMMAND", "CREATED", "STATUS", "PORTS")
		for _, c := range cons {
			cmd := c.Command
			if len(cmd) > 20 && !opts.Full {
				cmd = cmd[:20]
			}
			tw.Append(c.ID[:12], docker.CanonicalName(c.Names), c.Image, cmd,
				humanize.Time(time.Unix(c.Created, 0)), c.Status, docker.FormatPorts(c.Ports))
		}
		tw.Write(os.Stdout, "  ")
		fmt.Println()
	}
	return nil
}

type tableWriter struct {
	rows  [][]interface{}
	width []int
}

func (t *tableWriter) Append(cols ...string) {
	if t.width == nil {
		t.width = make([]int, len(cols))
	}
	if len(cols) != len(t.width) {
		panic("tableWriter: length mismatch")
	}
	var row []interface{}
	for i, col := range cols {
		row = append(row, col)
		if n := len(col); n > t.width[i] {
			t.width[i] = n
		}
	}
	t.rows = append(t.rows, row)
}

func (t *tableWriter) Write(w io.Writer, prefix string) {
	s := prefix
	for _, n := range t.width {
		s += "%-" + strconv.Itoa(n+3) + "s"
	}
	s += "\n"
	for _, row := range t.rows {
		fmt.Fprintf(w, s, row...)
	}
}

func init() {
	parser.AddCommand("ps", "List containers", "", &CmdPs{})
}
