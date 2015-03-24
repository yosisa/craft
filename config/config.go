package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Listen    string
	Docker    string
	AgentName string `toml:"agent_name"`
	Agents    []string
}

func Parse(path string) (*Config, error) {
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return nil, err
	}
	if c.Listen == "" {
		c.Listen = ":7300"
	}
	if c.Docker == "" {
		c.Docker = "unix:///var/run/docker.sock"
	}
	if c.AgentName == "" {
		c.AgentName, _ = os.Hostname()
	}
	if len(c.Agents) == 0 {
		c.Agents = append(c.Agents, "localhost:7300")
	}
	return &c, nil
}
