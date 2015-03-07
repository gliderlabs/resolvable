package resolver

import (
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

type Resolver interface {
	AddHost(id string, addr net.IP, name string, aliases ...string) error
	RemoveHost(id string) error
	Listen() error
	Close() error
}

type dnsmasqResolver struct {
	Port    int
	hosts   *Hosts
	dnsmasq *exec.Cmd
}

func NewDnsmasqResolver() *dnsmasqResolver {
	hosts := NewHosts("/tmp/hosts")
	return &dnsmasqResolver{hosts: hosts, Port: 53}
}

func (r *dnsmasqResolver) AddHost(id string, addr net.IP, name string, aliases ...string) error {
	if err := r.hosts.Add(id, addr, name, aliases...); err != nil {
		return err
	}
	return r.reload()
}

func (r *dnsmasqResolver) RemoveHost(id string) error {
	if err := r.hosts.Remove(id); err != nil {
		return err
	}
	return r.reload()
}

func (r *dnsmasqResolver) reload() error {
	if r.dnsmasq == nil {
		return nil
	}
	return r.dnsmasq.Process.Signal(syscall.SIGHUP)
}

func (r *dnsmasqResolver) Listen() error {
	r.dnsmasq = exec.Command("dnsmasq",
		"--port", strconv.Itoa(r.Port),
		"--no-daemon", "--no-hosts",
		"--addn-hosts", r.hosts.path,
		"--no-resolv",
		// --resolv-file our-resolv
	)
	r.dnsmasq.Stdout = os.Stdout
	r.dnsmasq.Stderr = os.Stderr

	err := r.dnsmasq.Start()
	if err != nil {
		return err
	}
	return nil
}

func (r *dnsmasqResolver) Close() error {
	// TODO clean up hosts file
	if r.dnsmasq != nil {
		return r.dnsmasq.Process.Kill()
	}
	return nil
}
