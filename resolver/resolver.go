package resolver

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

type Resolver interface {
	AddHost(id string, addr net.IP, name string, aliases ...string) error
	RemoveHost(id string) error

	AddUpstream(id string, addr net.IP, port int, domain ...string) error
	RemoveUpstream(id string) error

	Listen() error
	Close()
}

type dnsmasqResolver struct {
	Port      int
	configDir string
	hosts     *EntriesFile
	upstream  *EntriesFile
	dnsmasq   *exec.Cmd
}

func NewDnsmasqResolver() (*dnsmasqResolver, error) {
	configDir, err := ioutil.TempDir("", "docker-resolve")
	if err != nil {
		return nil, err
	}
	hosts := NewEntriesFile(filepath.Join(configDir, "hosts"))
	upstream := NewEntriesFile(filepath.Join(configDir, "upstream"))
	return &dnsmasqResolver{configDir: configDir, hosts: hosts, upstream: upstream, Port: 53}, nil
}

func (r *dnsmasqResolver) AddHost(id string, addr net.IP, name string, aliases ...string) error {
	if err := r.hosts.Add(id, NewHostsEntry(addr, name, aliases...)); err != nil {
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

func (r *dnsmasqResolver) AddUpstream(id string, addr net.IP, port int, domains ...string) error {
	if len(domains) == 0 {
		domains = []string{""}
	}

	entries := make([]fmt.Stringer, len(domains))
	for i, domain := range domains {
		entries[i] = NewServersEntry(addr, port, domain)
	}

	if err := r.upstream.Add(id, entries...); err != nil {
		return err
	}
	return r.reload()
}

func (r *dnsmasqResolver) RemoveUpstream(id string) error {
	if err := r.upstream.Remove(id); err != nil {
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
		"--servers-file", r.upstream.path,
		"--no-resolv",
	)
	r.dnsmasq.Stdout = os.Stdout
	r.dnsmasq.Stderr = os.Stderr

	err := r.dnsmasq.Start()
	if err != nil {
		return err
	}
	return nil
}

func (r *dnsmasqResolver) Close() {
	if r.dnsmasq != nil {
		r.dnsmasq.Process.Kill()
	}
	os.RemoveAll(r.configDir)
}
