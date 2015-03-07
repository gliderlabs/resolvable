package resolver

import (
	"log"
	"net"
	"os"
	"strings"
	"sync"
)

type HostsEntry struct {
	Address net.IP
	Names   []string
}

func (c *HostsEntry) ToHostsString() string {
	return strings.Join(append([]string{c.Address.String()}, c.Names...), "\t")
}

type Hosts struct {
	sync.Mutex
	path  string
	hosts map[string]*HostsEntry
}

func NewHosts(path string) *Hosts {
	h := &Hosts{path: path, hosts: make(map[string]*HostsEntry)}
	h.write()
	return h
}

func (h *Hosts) write() error {
	f, err := os.Create(h.path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, entry := range h.hosts {
		_, err := f.WriteString(entry.ToHostsString() + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *Hosts) Add(id string, addr net.IP, name string, aliases ...string) error {
	h.Lock()
	defer h.Unlock()

	names := append([]string{name}, aliases...)

	h.hosts[id] = &HostsEntry{Names: names, Address: addr}

	log.Println("added", id, "with value:", h.hosts[id].ToHostsString())

	return h.write()
}

func (h *Hosts) Remove(id string) error {
	h.Lock()
	defer h.Unlock()

	delete(h.hosts, id)

	log.Println("removed", id)

	return h.write()
}
