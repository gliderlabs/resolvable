package main // import "github.com/mgood/resolve"

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/mgood/resolve/resolver"

	dockerapi "github.com/fsouza/go-dockerclient"
)

var Version string

const RESOLVCONF_COMMENT = "# added by resolve"

var resolvConfPattern = regexp.MustCompile("(?m:^.*" + regexp.QuoteMeta(RESOLVCONF_COMMENT) + ")(?:$|\n)")

func getopt(name, def string) string {
	if env := os.Getenv(name); env != "" {
		return env
	}
	return def
}

func updateResolvConf(insert, path string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	orig, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	orig = resolvConfPattern.ReplaceAllLiteral(orig, []byte{})

	if _, err = f.Seek(0, os.SEEK_SET); err != nil {
		return err
	}

	if _, err = f.WriteString(insert); err != nil {
		return err
	}

	if _, err = f.Write(orig); err != nil {
		return err
	}

	// contents may have been shortened, so truncate where we are
	pos, err := f.Seek(0, os.SEEK_CUR)
	if err != nil {
		return err
	}
	return f.Truncate(pos)
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

func parseContainerEnv(containerEnv []string, prefix string) map[string]string {
	parsed := make(map[string]string)

	for _, env := range containerEnv {
		if !strings.HasPrefix(env, prefix) {
			continue
		}
		keyVal := strings.SplitN(env, "=", 2)
		if len(keyVal) > 1 {
			parsed[keyVal[0]] = keyVal[1]
		} else {
			parsed[keyVal[0]] = ""
		}
	}

	return parsed
}

func registerContainers(docker *dockerapi.Client, dns resolver.Resolver, containerDomain string) error {
	events := make(chan *dockerapi.APIEvents)
	if err := docker.AddEventListener(events); err != nil {
		return err
	}

	if !strings.HasPrefix(containerDomain, ".") {
		containerDomain = "." + containerDomain
	}

	addContainer := func(containerId string) error {
		container, err := docker.InspectContainer(containerId)
		if err != nil {
			return err
		}
		addr := net.ParseIP(container.NetworkSettings.IPAddress)

		err = dns.AddHost(containerId, addr, container.Config.Hostname, container.Name[1:]+containerDomain)
		if err != nil {
			return err
		}

		env := parseContainerEnv(container.Config.Env, "DNS_")
		if dnsDomains, ok := env["DNS_RESOLVES"]; ok {
			if dnsDomains == "" {
				return errors.New("empty DNS_RESOLVES, should contain a comma-separated list with at least one domain")
			}

			port := 53
			if portString := env["DNS_PORT"]; portString != "" {
				port, err = strconv.Atoi(portString)
				if err != nil {
					return errors.New("invalid DNS_PORT \"" + portString + "\", should contain a number")
				}
			}

			domains := strings.Split(dnsDomains, ",")
			err = dns.AddUpstream(containerId, addr, port, domains...)
			if err != nil {
				return err
			}
		}

		if bridge := container.NetworkSettings.Bridge; bridge != "" {
			bridgeAddr := net.ParseIP(container.NetworkSettings.Gateway)
			err = dns.AddHost("bridge:"+bridge, bridgeAddr, bridge)
			if err != nil {
				return err
			}
		}

		return nil
	}

	containers, err := docker.ListContainers(dockerapi.ListContainersOptions{})
	if err != nil {
		return err
	}

	for _, listing := range containers {
		// TODO report errors adding containers?
		addContainer(listing.ID)
	}

	if err = dns.Listen(); err != nil {
		return err
	}
	defer dns.Close()

	for msg := range events {
		switch msg.Status {
		case "start":
			go func() {
				if err := addContainer(msg.ID); err != nil {
					log.Printf("error adding container %s: %s\n", msg.ID[:12], err)
				}
			}()
		case "die":
			go func() {
				dns.RemoveHost(msg.ID)
				dns.RemoveUpstream(msg.ID)
			}()
		}
	}

	return errors.New("docker event loop closed")
}

func run() error {
	// set up the signal handler first to ensure cleanup is handled if a signal is
	// caught while initializing
	exitReason := make(chan error)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		sig := <-c
		exitReason <- errors.New(fmt.Sprint("terminated by signal ", sig))
	}()

	docker, err := dockerapi.NewClient(getopt("DOCKER_HOST", "unix:///tmp/docker.sock"))
	if err != nil {
		return err
	}

	address, err := ipAddress()
	if err != nil {
		return err
	}
	log.Println("got local address:", address)

	resolveConf := getopt("RESOLV_CONF", "/tmp/resolv.conf")
	resolveConfEntry := fmt.Sprintf("nameserver %s %s\n", address, RESOLVCONF_COMMENT)
	if err = updateResolvConf(resolveConfEntry, resolveConf); err != nil {
		return err
	}
	defer func() {
		log.Println("cleaning up", resolveConf)
		updateResolvConf("", resolveConf)
	}()

	dnsmasq, err := resolver.NewDnsmasqResolver()
	if err != nil {
		return err
	}
	defer dnsmasq.Close()

	dnsmasq.LocalDomain = "docker"

	go func() {
		dnsmasq.Wait()
		exitReason <- errors.New("dnsmasq process exited")
	}()
	go func() {
		exitReason <- registerContainers(docker, dnsmasq, dnsmasq.LocalDomain)
	}()

	return <-exitReason
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(Version)
		os.Exit(0)
	}
	log.Printf("Starting resolve %s ...", Version)

	err := run()
	if err != nil {
		log.Fatal("resolve: ", err)
	}
}
