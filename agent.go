package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/yosisa/craft/rpc"
)

type CmdAgent struct{}

func (opts *CmdAgent) Execute(args []string) error {
	conf := gopts.ParseConfig()
	if err := rpc.ListenAndServe(conf); err != nil {
		log.WithField("error", err).Fatal("Failed to listen")
	}
	return nil
}

func init() {
	parser.AddCommand("agent", "Run as agent mode", "", &CmdAgent{})
}
