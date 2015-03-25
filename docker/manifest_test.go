package docker

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/fsouza/go-dockerclient"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type ManifestSuite struct{}

var _ = Suite(&ManifestSuite{})

func (s *ManifestSuite) TestSplitImageTag(c *C) {
	m := &Manifest{Image: "go"}
	image, tag := m.SplitImageTag()
	c.Assert(image, Equals, "go")
	c.Assert(tag, Equals, "latest")

	m = &Manifest{Image: "go:1.4"}
	image, tag = m.SplitImageTag()
	c.Assert(image, Equals, "go")
	c.Assert(tag, Equals, "1.4")

	m = &Manifest{Image: "localhost:5000/myimage"}
	image, tag = m.SplitImageTag()
	c.Assert(image, Equals, "localhost:5000/myimage")
	c.Assert(tag, Equals, "latest")

	m = &Manifest{Image: "localhost:5000/myimage:beta"}
	image, tag = m.SplitImageTag()
	c.Assert(image, Equals, "localhost:5000/myimage")
	c.Assert(tag, Equals, "beta")
}

func (s *ManifestSuite) TestPort(c *C) {
	m := &Manifest{
		Ports: []PortSpec{
			{Exposed: "80/tcp"},
			{"443/tcp", "", 443},
		},
	}
	c.Assert(m.ExposedPorts(), DeepEquals, map[docker.Port]struct{}{
		"80/tcp":  struct{}{},
		"443/tcp": struct{}{},
	})
	c.Assert(m.PortBindings(), DeepEquals, map[docker.Port][]docker.PortBinding{
		"80/tcp":  {{"", "0"}},
		"443/tcp": {{"", "443"}},
	})
}

func (s *ManifestSuite) TestMergeEnv(c *C) {
	m := &Manifest{}
	m.MergeEnv(map[string]string{"ID": "1"})
	c.Assert(m.Env, DeepEquals, Env{"ID": "1"})

	m.MergeEnv(map[string]string{"TESTING": "yes"})
	c.Assert(m.Env, DeepEquals, Env{"ID": "1", "TESTING": "yes"})
}

type PortSpecSuite struct{}

var _ = Suite(&PortSpecSuite{})

func (s *PortSpecSuite) TestUnmarshal(c *C) {
	text := `["80/tcp", "80 -> 80/tcp", "127.0.0.1:80 -> 80/tcp"]`
	var ports []PortSpec
	err := json.Unmarshal([]byte(text), &ports)
	c.Assert(err, IsNil)
	c.Assert(ports, HasLen, 3)

	c.Assert(ports[0].Exposed, Equals, docker.Port("80/tcp"))
	c.Assert(ports[0].HostIP, Equals, "")
	c.Assert(ports[0].HostPort, Equals, int64(0))

	c.Assert(ports[1].Exposed, Equals, docker.Port("80/tcp"))
	c.Assert(ports[1].HostIP, Equals, "")
	c.Assert(ports[1].HostPort, Equals, int64(80))

	c.Assert(ports[2].Exposed, Equals, docker.Port("80/tcp"))
	c.Assert(ports[2].HostIP, Equals, "127.0.0.1")
	c.Assert(ports[2].HostPort, Equals, int64(80))
}

type VolumeSpecSuite struct{}

var _ = Suite(&VolumeSpecSuite{})

func (s *VolumeSpecSuite) TestUnmarshal(c *C) {
	text := `["/var/tmp", "/data -> /opt"]`
	var volumes []VolumeSpec
	err := json.Unmarshal([]byte(text), &volumes)
	c.Assert(err, IsNil)
	c.Assert(volumes, HasLen, 2)

	c.Assert(volumes[0].Path, Equals, "/var/tmp")
	c.Assert(volumes[0].Target, Equals, "/var/tmp")

	c.Assert(volumes[1].Path, Equals, "/data")
	c.Assert(volumes[1].Target, Equals, "/opt")
}

func (s *VolumeSpecSuite) TestString(c *C) {
	vs := VolumeSpec{Path: "/data", Target: "/opt"}
	c.Assert(vs.String(), Equals, "/data:/opt")
}

type LinkSuite struct{}

var _ = Suite(&LinkSuite{})

func (s *LinkSuite) TestUnmarshal(c *C) {
	var links []Link
	text := `["name", "name:alias"]`
	err := json.Unmarshal([]byte(text), &links)
	c.Assert(err, IsNil)
	c.Assert(links, HasLen, 2)

	c.Assert(links[0].Name, Equals, "name")
	c.Assert(links[0].Alias, Equals, "name")

	c.Assert(links[1].Name, Equals, "name")
	c.Assert(links[1].Alias, Equals, "alias")
}

func (s LinkSuite) TestString(c *C) {
	l := Link{Name: "name", Alias: "alias"}
	c.Assert(l.String(), Equals, "name:alias")
}

type EnvSuite struct{}

var _ = Suite(&EnvSuite{})

func (s *EnvSuite) TestPairs(c *C) {
	e := Env{"USER": "foo", "TESTING": "yes"}
	pairs := e.Pairs()
	sort.Strings(pairs)
	c.Assert(pairs, DeepEquals, []string{"TESTING=yes", "USER=foo"})
}
