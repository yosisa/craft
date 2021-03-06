package docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"

	"github.com/fsouza/go-dockerclient"
)

var (
	validImageTag    = regexp.MustCompile(`(.+?)(:[\w][\w.-]{0,127})?$`)
	validNetworkMode = regexp.MustCompile(`(bridge|none|host|container:[\w][\w.-]*)`)
)

type Manifest struct {
	Name        string
	Image       string
	ImageHash   string `json:"image_hash"`
	Ports       []PortSpec
	Mounts      []MountSpec
	Volumes     []string
	VolumesFrom []string `json:"volumes_from"`
	Links       []Link
	ExLinks     []Link
	Env         Env
	Cmd         []string
	DNS         []string
	NetworkMode string `json:"network_mode"`
	Restrict    Restrict
	StartWait   uint `json:"start_wait"`
	Replace     string
	ReplaceWait uint `json:"replace_wait"`
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
	for _, v := range m.Mounts {
		s = append(s, v.String())
	}
	return s
}

func (m *Manifest) VolumeMap() map[string]struct{} {
	if len(m.Volumes) == 0 {
		return nil
	}
	v := make(map[string]struct{})
	for _, volume := range m.Volumes {
		v[volume] = struct{}{}
	}
	return v
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

func (s *PortSpec) UnmarshalJSON(b []byte) (err error) {
	b = bytes.Trim(b, `"`)
	items := bytes.Split(b, []byte("->"))
	if len(items) == 1 {
		s.Exposed = docker.Port(bytes.TrimSpace(items[0]))
	} else {
		s.Exposed = docker.Port(bytes.TrimSpace(items[1]))
		parts := bytes.Split(bytes.TrimSpace(items[0]), []byte{':'})
		if len(parts) == 1 {
			s.HostPort, err = strconv.ParseInt(string(parts[0]), 10, 64)
		} else {
			s.HostIP = string(parts[0])
			s.HostPort, err = strconv.ParseInt(string(parts[1]), 10, 64)
		}
	}
	return
}

type MountSpec struct {
	Path   string
	Target string
}

func (s *MountSpec) UnmarshalJSON(b []byte) error {
	b = bytes.Trim(b, `"`)
	items := bytes.Split(b, []byte("->"))
	s.Path = string(bytes.TrimSpace(items[0]))
	if len(items) == 1 {
		s.Target = s.Path
	} else {
		s.Target = string(bytes.TrimSpace(items[1]))
	}
	return nil
}

func (s *MountSpec) String() string {
	return s.Path + ":" + s.Target
}

type Link struct {
	Name  string
	Alias string
}

func (l *Link) UnmarshalJSON(b []byte) error {
	b = bytes.Trim(b, `"`)
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
	Agent     string
	Labels    map[string]string
	Conflicts []string
}

func (r *Restrict) Validate() error {
	if _, err := regexp.Compile(r.Agent); err != nil {
		return err
	}
	for _, c := range r.Conflicts {
		if _, err := regexp.Compile(c); err != nil {
			return err
		}
	}
	return nil
}

func ParseManifest(path string) (*Manifest, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func SplitImageTag(image string) (string, string) {
	g := validImageTag.FindStringSubmatch(image)
	if g[2] != "" {
		return g[1], g[2][1:]
	}
	return g[1], "latest"
}
