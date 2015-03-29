package rpc

import (
	"github.com/fsouza/go-dockerclient"
	cdocker "github.com/yosisa/craft/docker"
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

func (d *Docker) StartContainer(req string, resp *Empty) error {
	return d.c.StartContainer(req, nil)
}

type RemoveContainerRequest struct {
	ID    string
	Force bool
}

func (d *Docker) RemoveContainer(req RemoveContainerRequest, resp *Empty) error {
	return d.c.RemoveContainer(docker.RemoveContainerOptions{
		ID:    req.ID,
		Force: req.Force,
	})
}

type PullImageRequest struct {
	Image    string
	StreamID uint32
}

func (d *Docker) PullImage(req PullImageRequest, resp *Empty) error {
	w, err := streamConn.get(req.StreamID)
	if err != nil {
		return err
	}
	defer w.Close()
	image, tag := cdocker.SplitImageTag(req.Image)
	opts := docker.PullImageOptions{
		Repository:    image,
		Tag:           tag,
		OutputStream:  w,
		RawJSONStream: true,
	}
	auth := docker.AuthConfiguration{}
	return d.c.PullImage(opts, auth)
}
