package docker

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/fsouza/go-dockerclient"
)

var (
	validImageTag    = regexp.MustCompile(`(.+?)(:[\w][\w.-]{0,127})?$`)
	validNetworkMode = regexp.MustCompile(`(bridge|none|host|container:[\w][\w.-]*)`)
)

type Manifest struct {
	Name        string
	Image       string
	ImageHash   string `toml:"image_hash"`
	Ports       []PortSpec
	Volumes     []VolumeSpec
	Links       []Link
	ExLinks     []Link
	Env         Env
	Cmd         []string
	DNS         []string
	NetworkMode string `toml:"network_mode"`
	Restrict    Restrict
	StartWait   uint `toml:"start_wait"`
	Replace     string
	ReplaceWait uint `toml:"replace_wait"`
}

func (m *Manifest) Validate() error {
	if !validImageTag.MatchString(m.Image) {
		return fmt.Errorf("Invalid image name: %s", m.Image)
	}
	if m.NetworkMode != "" && !validNetworkMode.MatchString(m.NetworkMode) {
		return fmt.Errorf("Invalid network mode: %s", m.NetworkMode)
	}
	if m.ReplaceWait == 0 {
		m.ReplaceWait = 10
	}
	return m.Restrict.Validate()
}

func (m *Manifest) SplitImageTag() (string, string) {
	g := validImageTag.FindStringSubmatch(m.Image)
	if g[2] != "" {
		return g[1], g[2][1:]
	}
	return g[1], "latest"
}

func (m *Manifest) ExposedPorts() map[docker.Port]struct{} {
	if len(m.Ports) == 0 {
		return nil
	}
	ep := make(map[docker.Port]struct{})
	for _, p := range m.Ports {
		ep[p.Exposed] = struct{}{}
	}
	return ep
}

func (m *Manifest) PortBindings() map[docker.Port][]docker.PortBinding {
	if len(m.Ports) == 0 {
		return nil
	}
	pb := make(map[docker.Port][]docker.PortBinding)
	for _, p := range m.Ports {
		pb[p.Exposed] = append(pb[p.Exposed], docker.PortBinding{
			HostIP:   p.HostIP,
			HostPort: fmt.Sprintf("%d", p.HostPort),
		})
	}
	return pb
}

func (m *Manifest) Binds() []string {
	var s []string
	for _, v := range m.Volumes {
		s = append(s, v.String())
	}
	return s
}

func (m *Manifest) LinkList() []string {
	var s []string
	for _, v := range m.Links {
		s = append(s, v.String())
	}
	return s
}

func (m *Manifest) MergeEnv(env map[string]string) {
	if m.Env == nil {
		m.Env = make(Env)
	}
	for k, v := range env {
		m.Env[k] = v
	}
}

type PortSpec struct {
	Exposed  docker.Port
	HostIP   string
	HostPort int64
}

func (s *PortSpec) UnmarshalText(b []byte) (err error) {
	items := bytes.Split(b, []byte("->"))
	if len(items) == 1 {
		s.Exposed = docker.Port(bytes.Trim(items[0], " "))
	} else {
		s.Exposed = docker.Port(bytes.Trim(items[1], " "))
		parts := bytes.Split(bytes.Trim(items[0], " "), []byte{':'})
		if len(parts) == 1 {
			s.HostPort, err = strconv.ParseInt(string(parts[0]), 10, 64)
		} else {
			s.HostIP = string(parts[0])
			s.HostPort, err = strconv.ParseInt(string(parts[1]), 10, 64)
		}
	}
	return
}

type VolumeSpec struct {
	Path   string
	Target string
}

func (s *VolumeSpec) UnmarshalText(b []byte) error {
	items := bytes.Split(b, []byte("->"))
	s.Path = string(bytes.Trim(items[0], " "))
	if len(items) == 1 {
		s.Target = s.Path
	} else {
		s.Target = string(bytes.Trim(items[1], " "))
	}
	return nil
}

func (s *VolumeSpec) String() string {
	return s.Path + ":" + s.Target
}

type Link struct {
	Name  string
	Alias string
}

func (l *Link) UnmarshalText(b []byte) error {
	items := bytes.Split(b, []byte(":"))
	l.Name = string(items[0])
	if len(items) == 1 {
		l.Alias = l.Name
	} else {
		l.Alias = string(items[1])
	}
	return nil
}

func (l *Link) String() string {
	return l.Name + ":" + l.Alias
}

type Env map[string]string

func (e Env) Pairs() []string {
	var pairs []string
	for k, v := range e {
		pairs = append(pairs, k+"="+v)
	}
	return pairs
}

type Restrict struct {
	Agent string
}

func (r *Restrict) Validate() error {
	if _, err := regexp.Compile(r.Agent); err != nil {
		return err
	}
	return nil
}

func ParseManifest(path string) (*Manifest, error) {
	var m Manifest
	if _, err := toml.DecodeFile(path, &m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}
