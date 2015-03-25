package docker

import (
	"sort"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/fsouza/go-dockerclient"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

var pspec = `
ports = ["80/tcp", "80 -> 80/tcp", "127.0.0.1:80 -> 80/tcp"]
`

var vspec = `
volumes = ["/var/tmp", "/data -> /opt"]
`

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
	var ps struct{ Ports []PortSpec }
	_, err := toml.Decode(pspec, &ps)
	c.Assert(err, IsNil)

	c.Assert(ps.Ports[0].Exposed, Equals, docker.Port("80/tcp"))
	c.Assert(ps.Ports[0].HostIP, Equals, "")
	c.Assert(ps.Ports[0].HostPort, Equals, int64(0))

	c.Assert(ps.Ports[1].Exposed, Equals, docker.Port("80/tcp"))
	c.Assert(ps.Ports[1].HostIP, Equals, "")
	c.Assert(ps.Ports[1].HostPort, Equals, int64(80))

	c.Assert(ps.Ports[2].Exposed, Equals, docker.Port("80/tcp"))
	c.Assert(ps.Ports[2].HostIP, Equals, "127.0.0.1")
	c.Assert(ps.Ports[2].HostPort, Equals, int64(80))
}

type VolumeSpecSuite struct{}

var _ = Suite(&VolumeSpecSuite{})

func (s *VolumeSpecSuite) TestUnmarshal(c *C) {
	var vs struct{ Volumes []VolumeSpec }
	_, err := toml.Decode(vspec, &vs)
	c.Assert(err, IsNil)

	c.Assert(vs.Volumes[0].Path, Equals, "/var/tmp")
	c.Assert(vs.Volumes[0].Target, Equals, "/var/tmp")

	c.Assert(vs.Volumes[1].Path, Equals, "/data")
	c.Assert(vs.Volumes[1].Target, Equals, "/opt")
}

func (s *VolumeSpecSuite) TestString(c *C) {
	vs := VolumeSpec{Path: "/data", Target: "/opt"}
	c.Assert(vs.String(), Equals, "/data:/opt")
}

type LinkSuite struct{}

var _ = Suite(&LinkSuite{})

func (s *LinkSuite) TestUnmarshal(c *C) {
	var v struct{ L []Link }
	text := `l = ["name", "name:alias"]` + "\n"
	_, err := toml.Decode(text, &v)
	c.Assert(err, IsNil)

	c.Assert(v.L[0].Name, Equals, "name")
	c.Assert(v.L[0].Alias, Equals, "name")

	c.Assert(v.L[1].Name, Equals, "name")
	c.Assert(v.L[1].Alias, Equals, "alias")
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
