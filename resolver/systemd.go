package resolver

import (
	"fmt"
	"io/ioutil"
)

type SystemdNetworkConfig struct {
	path string
}

func init() {
	systemdConf := getopt("SYSTEMD_NETWORK_CONF", "/tmp/systemd.network")
	HostResolverConfigs.Register(&SystemdNetworkConfig{systemdConf}, "systemd")
}

func (r *SystemdNetworkConfig) StoreAddress(address string) error {
	systemdEntry := fmt.Sprintf("[Match]\nName=*\n[Network]\nDNS=%s\n", address)
	return ioutil.WriteFile(r.path, []byte(systemdEntry), 0664)
}

func (r *SystemdNetworkConfig) Clean() {
	ioutil.WriteFile(r.path, []byte{}, 0664)
}
