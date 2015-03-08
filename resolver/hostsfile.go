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
	Domain  string
}

func NewServersEntry(addr net.IP, port int, domain string) *ServersEntry {
	return &ServersEntry{addr, port, domain}
}

func (s *ServersEntry) String() string {
	if s.Domain == "" {
		return fmt.Sprintf("server=%v#%d", s.Address, s.Port)
	} else {
		return fmt.Sprintf("server=/%v/%v#%d", s.Domain, s.Address, s.Port)
	}
}

type EntriesFile struct {
	sync.Mutex
	path    string
	entries map[string][]fmt.Stringer
}

func NewEntriesFile(path string) *EntriesFile {
	h := &EntriesFile{path: path, entries: make(map[string][]fmt.Stringer)}
	h.write()
	return h
}

func (h *EntriesFile) write() error {
	f, err := os.Create(h.path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, entries := range h.entries {
		for _, entry := range entries {
			_, err := f.WriteString(entry.String() + "\n")
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (h *EntriesFile) Add(id string, entries ...fmt.Stringer) error {
	h.Lock()
	defer h.Unlock()

	h.entries[id] = entries

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
