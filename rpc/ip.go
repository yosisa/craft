package rpc

import (
	"net"
	"sort"
)

func ListIPAddrs() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	for _, iface := range ifaces {
		if iface.Name == "docker0" {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}
		for _, addr := range addrs {
			ip, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ip.IP.IsLoopback() || ip.IP.IsMulticast() {
				continue
			}
			ips = append(ips, ip.IP)
		}
	}
	sort.Sort(ByNetworkType(ips))
	out := make([]string, len(ips))
	for i, ip := range ips {
		out[i] = ip.String()
	}
	return out, nil
}

var networkOrder []*net.IPNet

type ByNetworkType []net.IP

func (s ByNetworkType) Len() int {
	return len(s)
}

func (s ByNetworkType) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ByNetworkType) Less(i, j int) bool {
	return s.indexOf(s[i]) < s.indexOf(s[j])
}

func (s ByNetworkType) indexOf(ip net.IP) int {
	for i, ipnet := range networkOrder {
		if ipnet.Contains(ip) {
			return i
		}
	}
	return len(networkOrder)
}

func init() {
	for _, s := range []string{
		"192.168.0.0/16",
		"172.16.0.0/12",
		"10.0.0.0/8",
		"169.254.0.0/16",
		"0.0.0.0/0",
	} {
		_, ipnet, err := net.ParseCIDR(s)
		if err != nil {
			panic(err)
		}
		networkOrder = append(networkOrder, ipnet)
	}
}
