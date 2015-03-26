package rpc

import (
	"github.com/fsouza/go-dockerclient"
)

type Docker struct {
	c *docker.Client
}

func NewDocker(endpoint string) (*Docker, error) {
	c, err := docker.NewClient(endpoint)
	if err != nil {
		return nil, err
	}
	return &Docker{c: c}, nil
}

type ListContainersRequest struct {
	All bool
}

type ListContainersResponse struct {
	Containers []docker.APIContainers
}

func (d *Docker) ListContainers(req ListContainersRequest, resp *ListContainersResponse) error {
	cons, err := d.c.ListContainers(docker.ListContainersOptions{All: req.All})
	if err != nil {
		return err
	}
	resp.Containers = cons
	return nil
}
