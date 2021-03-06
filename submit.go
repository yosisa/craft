package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/yosisa/craft/docker"
	"github.com/yosisa/craft/rpc"
)

type CmdSubmit struct {
	NameSuffix    string `long:"name-suffix" description:"Append to name value"`
	ReplaceSuffix string `long:"replace-suffix" description:"Append to replace value"`
	Args          struct {
		Manifest string `positional-arg-name:"MANIFEST"`
	} `positional-args:"yes" required:"yes"`
}

func (opts *CmdSubmit) Execute(args []string) error {
	m, err := docker.ParseManifest(opts.Args.Manifest)
	if err != nil {
		log.WithField("error", err).Fatal("Could not parse manifest")
	}
	m.Name += opts.NameSuffix
	m.Replace += opts.ReplaceSuffix

	caps := gatherCapabilities(gopts.agents())
	agent := findBestAgent(m, caps.Copy())
	if agent == "" {
		log.WithField("error", "No available agents").Fatal("Could not find best agent")
	}
	exlinks, err := resolveExLinks(m, caps)
	if err != nil {
		log.WithField("error", err).Fatal("Failed to resolve exlinks")
	}

	resp, err := rpc.Submit(agent, m, exlinks)
	if err != nil {
		log.WithField("error", err).Fatal("RPC failed")
	}
	log.WithFields(log.Fields{"name": m.Name, "agent": resp.Agent}).Info("Container running")
	return nil
}

func init() {
	parser.AddCommand("submit", "Run a container by the manifest", "", &CmdSubmit{})
}
