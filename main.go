package main // import "github.com/mgood/docker-resolver"

// dnsmasq --no-daemon --no-hosts --addn-hosts our-hosts --resolv-file our-resolv
import (
	"errors"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"

	dockerapi "github.com/fsouza/go-dockerclient"
)

type ContainerEntry struct {
	Address string
	Names   []string
}

func (c *ContainerEntry) ToHostsString() string {
	return strings.Join(append([]string{c.Address}, c.Names...), "\t")
}

type Hosts struct {
	sync.Mutex
	containers map[string]*ContainerEntry
	docker     *dockerapi.Client
	path       string
}

func NewHosts(docker *dockerapi.Client, path string) *Hosts {
	containers := make(map[string]*ContainerEntry)
	h := &Hosts{docker: docker, containers: containers, path: path}
	h.write()
	return h
}

func (h *Hosts) write() error {
	f, err := os.Create(h.path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, entry := range h.containers {
		_, err := f.WriteString(entry.ToHostsString() + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *Hosts) Add(containerId string) error {
	h.Lock()
	defer h.Unlock()

	container, err := h.docker.InspectContainer(containerId)
	if err != nil {
		return err
	}
	names := []string{container.Config.Hostname, container.Name[1:]}
	addr := container.NetworkSettings.IPAddress

	h.containers[containerId] = &ContainerEntry{Names: names, Address: addr}

	log.Println("added", containerId[:12], "with value:", h.containers[containerId].ToHostsString())

	return h.write()
}

func (h *Hosts) Remove(containerId string) error {
	h.Lock()
	defer h.Unlock()

	delete(h.containers, containerId)

	log.Println("removed", containerId[:12])

	return h.write()
}

func getopt(name, def string) string {
	if env := os.Getenv(name); env != "" {
		return env
	}
	return def
}

func assert(err error) {
	if err != nil {
		log.Fatal("dns: ", err)
	}
}

func insertLine(line, path string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	orig, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	if _, err = f.Seek(0, 0); err != nil {
		return err
	}

	if _, err = f.WriteString(line + "\n"); err != nil {
		return err
	}

	_, err = f.Write(orig)
	return err
}

func removeLine(text, path string) error {
	patt := regexp.MustCompile("(?m:^" + regexp.QuoteMeta(text) + ")(?:$|\n)")

	orig, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, patt.ReplaceAllLiteral(orig, []byte{}), 0666)
	return err
}

func ipAddress() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && !ipnet.IP.IsMulticast() {
			if ipv4 := ipnet.IP.To4(); ipv4 != nil {
				return ipv4.String(), nil
			}
		}
	}

	return "", errors.New("no addresses found")
}

func main() {
	// if len(os.Args) == 2 && os.Args[1] == "--version" {
	// 	fmt.Println(Version)
	// 	os.Exit(0)
	// }
	docker, err := dockerapi.NewClient(getopt("DOCKER_HOST", "unix:///tmp/docker.sock"))
	assert(err)

	// address, err := ipAddress()
	// assert(err)
	// log.Println("got local address:", address)

	// resolveConf := getopt("RESOLV_CONF", "/tmp/resolv.conf")
	// resolveConfEntry := "nameserver " + address
	// assert(insertLine(resolveConfEntry, resolveConf))
	// defer removeLine(resolveConfEntry, resolveConf)

	hosts := NewHosts(docker, "/tmp/hosts")

	dnsmasq := exec.Command("dnsmasq", "--no-daemon", "--no-hosts", "--addn-hosts", hosts.path)
	// --resolv-file our-resolv
	dnsmasq.Stdout = os.Stdout
	dnsmasq.Stderr = os.Stderr

	events := make(chan *dockerapi.APIEvents)
	assert(docker.AddEventListener(events))

	containers, err := docker.ListContainers(dockerapi.ListContainersOptions{})
	assert(err)

	for _, listing := range containers {
		hosts.Add(listing.ID)
	}

	assert(dnsmasq.Start())

	for msg := range events {
		switch msg.Status {
		case "start":
			go func() {
				hosts.Add(msg.ID)
				dnsmasq.Process.Signal(syscall.SIGHUP)
			}()
		case "die":
			go func() {
				hosts.Remove(msg.ID)
				dnsmasq.Process.Signal(syscall.SIGHUP)
			}()
		}
	}

	dnsmasq.Process.Kill()

	log.Fatal("dns: docker event loop closed") // todo: reconnect?
}
