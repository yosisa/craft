package docker

import (
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
)

type UsageInfo struct {
	AllNames   []string
	UsedNames  []string
	UsedPorts  []int64
	Containers map[string]*ContainerInfo
}

type ContainerInfo struct {
	Ports []int64
}

type Client struct {
	c *docker.Client
}

func NewClient(endpoint string) (*Client, error) {
	c, err := docker.NewClient(endpoint)
	if err != nil {
		return nil, err
	}
	return &Client{c: c}, nil
}

func (c *Client) Usage() (*UsageInfo, error) {
	cons, err := c.c.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		return nil, err
	}
	var ui UsageInfo
	ui.Containers = make(map[string]*ContainerInfo)
	for _, c := range cons {
		name := canonicalName(c.Names)
		ui.AllNames = append(ui.AllNames, name)
		if !strings.HasPrefix(c.Status, "Up") {
			continue
		}
		ui.UsedNames = append(ui.UsedNames, name)
		var ports []int64
		for _, p := range c.Ports {
			if p.PublicPort > 0 {
				ports = append(ports, p.PublicPort)
			}
		}
		ui.UsedPorts = append(ui.UsedPorts, ports...)
		ui.Containers[name] = &ContainerInfo{Ports: ports}
	}
	return &ui, nil
}

func (c *Client) Run(m *Manifest) error {
	image, tag := m.SplitImageTag()
	if hash := c.ImageHash(image, tag); hash == "" || !strings.HasPrefix(hash, m.ImageHash) {
		if err := c.PullImage(image, tag); err != nil {
			return err
		}
	}
	if m.Name == m.Replace {
		c.Remove(m.Name, m.ReplaceWait)
	}
	con, err := c.c.CreateContainer(docker.CreateContainerOptions{
		Name: m.Name,
		Config: &docker.Config{
			Image:        m.Image,
			Env:          m.Env.Pairs(),
			Cmd:          m.Cmd,
			ExposedPorts: m.ExposedPorts(),
		},
	})
	if err != nil {
		return err
	}
	if m.Replace != "" {
		c.Stop(m.Replace, m.ReplaceWait)
	}
	err = c.c.StartContainer(con.ID, &docker.HostConfig{
		Binds:        m.Binds(),
		PortBindings: m.PortBindings(),
		DNS:          m.DNS,
	})
	if err != nil || m.StartWait == 0 {
		return err
	}
	time.Sleep(time.Duration(m.StartWait) * time.Second)
	if con, err = c.c.InspectContainer(con.ID); err != nil {
		return err
	}
	if !con.State.Running {
		return &docker.ContainerNotRunning{ID: con.ID}
	}
	return nil
}

func (c *Client) ImageHash(name, tag string) string {
	img, err := c.c.InspectImage(name + ":" + tag)
	if err != nil {
		return ""
	}
	return img.ID
}

func (c *Client) PullImage(name, tag string) error {
	opts := docker.PullImageOptions{Repository: name, Tag: tag}
	auth := docker.AuthConfiguration{}
	return c.c.PullImage(opts, auth)
}

func (c *Client) Stop(name string, wait uint) error {
	err := c.c.StopContainer(name, wait)
	switch err.(type) {
	case *docker.NoSuchContainer:
	case *docker.ContainerNotRunning:
	default:
		return err
	}
	return nil
}

func (c *Client) Remove(name string, wait uint) error {
	c.Stop(name, wait)
	opts := docker.RemoveContainerOptions{ID: name, Force: true}
	err := c.c.RemoveContainer(opts)
	if err == nil {
		return nil
	}
	if _, ok := err.(*docker.NoSuchContainer); ok {
		return nil
	}
	return err
}

func canonicalName(ss []string) string {
	for _, s := range ss {
		s = s[1:]
		if !strings.Contains(s, "/") {
			return s
		}
	}
	return ""
}
