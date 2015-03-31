package rpc

import (
	"io"

	"github.com/fsouza/go-dockerclient"
	"github.com/pierrec/lz4"
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

type StopContainerRequest struct {
	ID      string
	Timeout uint
}

func (d *Docker) StopContainer(req StopContainerRequest, resp *Empty) error {
	return d.c.StopContainer(req.ID, req.Timeout)
}

type RestartContainerRequest struct {
	ID      string
	Timeout uint
}

func (d *Docker) RestartContainer(req RestartContainerRequest, resp *Empty) error {
	return d.c.RestartContainer(req.ID, req.Timeout)
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

type LogsRequest struct {
	Container   string
	Follow      bool
	Tail        string
	OutStreamID uint32
	ErrStreamID uint32
}

func (d *Docker) Logs(req LogsRequest, resp *Empty) error {
	oc, err := streamConn.get(req.OutStreamID)
	if err != nil {
		return err
	}
	defer oc.Close()
	ec, err := streamConn.get(req.ErrStreamID)
	if err != nil {
		return err
	}
	defer ec.Close()
	return d.c.Logs(docker.LogsOptions{
		Container:    req.Container,
		OutputStream: oc,
		ErrorStream:  ec,
		Follow:       req.Follow,
		Stdout:       true,
		Stderr:       true,
		Tail:         req.Tail,
	})
}

type LoadImageRequest struct {
	StreamID uint32
	Compress bool
	Rest     []string
}

func (d *Docker) LoadImage(req LoadImageRequest, resp *Empty) error {
	c, err := streamConn.get(req.StreamID)
	if err != nil {
		return err
	}
	var r io.Reader = c
	if len(req.Rest) == 0 {
		if req.Compress {
			r = lz4.NewReader(r)
		}
		return d.c.LoadImage(docker.LoadImageOptions{InputStream: r})
	}

	// pipelining and is intermediate node
	errc := make(chan error, 2)
	pr, pw := io.Pipe()
	r = io.TeeReader(r, pw)
	if req.Compress {
		r = lz4.NewReader(r)
	}
	go func() {
		errc <- connectImagePipeline(req.Rest, pr, req.Compress)
	}()
	go func() {
		errc <- d.c.LoadImage(docker.LoadImageOptions{InputStream: r})
		pw.Close()
	}()

	for i := 0; i < 2; i++ {
		if err == nil {
			err = <-errc
		} else {
			<-errc
		}
	}
	return err
}
