package resolver

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
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
	path    string
	entries map[string]string
}

func NewEntriesFile(path string) *EntriesFile {
	h := &EntriesFile{path: path, entries: make(map[string]string)}
	h.Write()
	return h
}

func (h *EntriesFile) Write() error {
	f, err := os.Create(h.path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, entry := range h.entries {
		_, err := f.WriteString(entry + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *EntriesFile) Add(id string, entry fmt.Stringer) bool {
	value := entry.String()
	if h.entries[id] == value {
		return false
	}

	h.entries[id] = value
	log.Println("added", id, "with value:", value)

	return true
}

func (h *EntriesFile) Remove(id string) bool {
	if _, ok := h.entries[id]; !ok {
		return false
	}

	delete(h.entries, id)
	log.Println("removed", id)

	return true
}
