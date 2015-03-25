package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

type Config struct {
	Listen    string
	Docker    string
	AgentName string `json:"agent_name"`
	Agents    []string
}

func Parse(path string) (*Config, error) {
	var c Config
	if path != "" {
		b, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &c); err != nil {
			return nil, err
		}
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
