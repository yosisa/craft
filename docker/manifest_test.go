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

type EnvSuite struct{}

var _ = Suite(&EnvSuite{})

func (s *EnvSuite) TestPairs(c *C) {
	e := Env{"USER": "foo", "TESTING": "yes"}
	pairs := e.Pairs()
	sort.Strings(pairs)
	c.Assert(pairs, DeepEquals, []string{"TESTING=yes", "USER=foo"})
}
