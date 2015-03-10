package resolver

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
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
	sync.Mutex
	Port        int
	LocalDomain string
	configDir   string
	hosts       *EntriesFile
	upstream    *EntriesFile
	dnsmasq     *exec.Cmd
	started     chan struct{}
}

func NewDnsmasqResolver() (*dnsmasqResolver, error) {
	configDir, err := ioutil.TempDir("", "docker-resolve")
	if err != nil {
		return nil, err
	}
	hosts := NewEntriesFile(filepath.Join(configDir, "hosts"))
	upstream := NewEntriesFile(filepath.Join(configDir, "upstream"))
	return &dnsmasqResolver{
		Port:      53,
		configDir: configDir,
		hosts:     hosts,
		upstream:  upstream,
		started:   make(chan struct{}),
	}, nil
}

func (r *dnsmasqResolver) AddHost(id string, addr net.IP, name string, aliases ...string) error {
	return r.addTo(r.hosts, id, NewHostsEntry(addr, name, aliases...))
}

func (r *dnsmasqResolver) RemoveHost(id string) error {
	return r.removeFrom(r.hosts, id)
}

func (r *dnsmasqResolver) AddUpstream(id string, addr net.IP, port int, domains ...string) error {
	return r.addTo(r.upstream, id, NewServersEntry(addr, port, domains...))
}

func (r *dnsmasqResolver) RemoveUpstream(id string) error {
	return r.removeFrom(r.upstream, id)
}

func (r *dnsmasqResolver) addTo(entries *EntriesFile, id string, entry fmt.Stringer) error {
	r.Lock()
	defer r.Unlock()

	if !entries.Add(id, entry) {
		return nil
	}
	return r.reload(entries)
}

func (r *dnsmasqResolver) removeFrom(entries *EntriesFile, id string) error {
	r.Lock()
	defer r.Unlock()

	if !entries.Remove(id) {
		return nil
	}
	return r.reload(entries)
}

func (r *dnsmasqResolver) reload(entries *EntriesFile) error {
	if err := entries.Write(); err != nil {
		return err
	}
	if r.dnsmasq == nil {
		return nil
	}
	return r.dnsmasq.Process.Signal(syscall.SIGHUP)
}

func (r *dnsmasqResolver) Listen() error {
	args := []string{
		"--port", strconv.Itoa(r.Port),
		"--no-daemon", "--no-hosts",
		"--addn-hosts", r.hosts.path,
		"--servers-file", r.upstream.path,
		"--no-resolv",
	}
	if r.LocalDomain != "" {
		args = append(args, "--local", "/"+r.LocalDomain+"/")
	}
	r.dnsmasq = exec.Command("dnsmasq", args...)
	r.dnsmasq.Stdout = os.Stdout
	r.dnsmasq.Stderr = os.Stderr

	err := r.dnsmasq.Start()
	close(r.started)
	return err
}

func (r *dnsmasqResolver) Wait() error {
	<-r.started
	return r.dnsmasq.Wait()
}

func (r *dnsmasqResolver) Close() {
	if r.dnsmasq != nil {
		r.dnsmasq.Process.Kill()
	}
	os.RemoveAll(r.configDir)
}
