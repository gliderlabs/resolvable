package resolver

import (
	"fmt"
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

func NewHostsEntry(addr net.IP, name string, aliases ...string) *HostsEntry {
	names := append([]string{name}, aliases...)
	return &HostsEntry{Names: names, Address: addr}
}

func (h *HostsEntry) String() string {
	return strings.Join(append([]string{h.Address.String()}, h.Names...), "\t")
}

type ServersEntry struct {
	Address net.IP
	Port    int
	Domains []string
}

func NewServersEntry(addr net.IP, port int, domains ...string) *ServersEntry {
	return &ServersEntry{addr, port, domains}
}

func (s *ServersEntry) String() string {
	domains := ""
	if len(s.Domains) > 0 {
		domains = "/" + strings.Join(s.Domains, "/") + "/"
	}
	return fmt.Sprintf("server=%s%v#%d", domains, s.Address, s.Port)
}

type EntriesFile struct {
	sync.Mutex
	path    string
	entries map[string]fmt.Stringer
}

func NewEntriesFile(path string) *EntriesFile {
	h := &EntriesFile{path: path, entries: make(map[string]fmt.Stringer)}
	h.write()
	return h
}

func (h *EntriesFile) write() error {
	f, err := os.Create(h.path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, entry := range h.entries {
		_, err := f.WriteString(entry.String() + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *EntriesFile) Add(id string, entry fmt.Stringer) error {
	h.Lock()
	defer h.Unlock()

	h.entries[id] = entry

	log.Println("added", id, "with value:", h.entries[id])

	return h.write()
}

func (h *EntriesFile) Remove(id string) error {
	h.Lock()
	defer h.Unlock()

	delete(h.entries, id)

	log.Println("removed", id)

	return h.write()
}
