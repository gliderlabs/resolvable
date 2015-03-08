package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/cenkalti/backoff"
	dockerapi "github.com/fsouza/go-dockerclient"
)

func TestStartupShutdown(t *testing.T) {
	t.Parallel()

	daemon, err := NewDaemon()
	ok(t, err)
	defer daemon.Close()

	dns := &DebugResolver{make(chan string)}
	go registerContainers(daemon.Client, dns)

	equals(t, "listen", <-dns.ch)

	ok(t, daemon.Close())
	equals(t, "close", <-dns.ch)
}

func TestAddContainerBeforeStarted(t *testing.T) {
	t.Parallel()

	daemon, err := NewDaemon()
	ok(t, err)
	defer daemon.Close()

	containerId, err := daemon.RunSimple("sleep", "30")
	ok(t, err)

	dns := &DebugResolver{make(chan string)}
	go registerContainers(daemon.Client, dns)

	equals(t, "add: "+containerId, <-dns.ch)
	equals(t, "listen", <-dns.ch)
}

func TestAddRemoveWhileRunning(t *testing.T) {
	t.Parallel()

	daemon, err := NewDaemon()
	ok(t, err)
	defer daemon.Close()

	dns := &DebugResolver{make(chan string)}
	go registerContainers(daemon.Client, dns)

	equals(t, "listen", <-dns.ch)

	containerId, err := daemon.RunSimple("sleep", "30")
	ok(t, err)

	equals(t, "add: "+containerId, <-dns.ch)

	ok(t, daemon.Client.KillContainer(dockerapi.KillContainerOptions{
		ID: containerId,
	}))

	equals(t, "remove: "+containerId, <-dns.ch)
}

// TODO add a test for when the container doesn't start up right,
// the IP will be nil, since the container aborted, so we shouldn't try to add it at all

////////////////////////////////////////////////////////////////////////////////
// Helpers
////////////////////////////////////////////////////////////////////////////////

type DockerDaemon struct {
	Client          *dockerapi.Client
	rootClient      *dockerapi.Client
	dindContainerId string
}

// ok fails the test if an err is not nil.
func ok(tb testing.TB, err error) {
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: unexpected error: %s\033[39m\n\n", filepath.Base(file), line, err.Error())
		tb.FailNow()
	}
}

// equals fails the test if exp is not equal to act.
func equals(tb testing.TB, exp, act interface{}) {
	if !reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\texp: %#v\n\n\tgot: %#v\033[39m\n\n", filepath.Base(file), line, exp, act)
		tb.FailNow()
	}
}

func clientFromEnv() (client *dockerapi.Client, endpointUrl *url.URL, err error) {
	endpoint := getopt("DOCKER_HOST", "unix:///var/run/docker.sock")
	endpointUrl, err = url.Parse(endpoint)
	if err != nil {
		return
	}

	if os.Getenv("DOCKER_TLS_VERIFY") == "" {
		client, err = dockerapi.NewClient(endpoint)
	} else {
		certPath := os.Getenv("DOCKER_CERT_PATH")
		client, err = dockerapi.NewTLSClient(endpoint,
			filepath.Join(certPath, "cert.pem"),
			filepath.Join(certPath, "key.pem"),
			filepath.Join(certPath, "ca.pem"),
		)
	}

	return
}

func NewDaemon() (daemon *DockerDaemon, err error) {
	rootClient, endpoint, err := clientFromEnv()
	if err != nil {
		return nil, err
	}

	// TODO share /var/lib/docker across the docker runs, but where should the volume be stored?

	daemon = &DockerDaemon{
		rootClient: rootClient,
	}
	defer func() {
		// if there is an error, client will not be set, so clean up
		if daemon.Client == nil {
			daemon.Close()
		}
	}()

	port := dockerapi.Port("4444/tcp")

	daemon.dindContainerId, err = runContainer(rootClient,
		dockerapi.CreateContainerOptions{
			Config: &dockerapi.Config{
				Image:        "jpetazzo/dind",
				Env:          []string{"PORT=" + port.Port()},
				ExposedPorts: map[dockerapi.Port]struct{}{port: {}},
			},
		}, &dockerapi.HostConfig{
			Privileged:      true,
			PublishAllPorts: true,
		},
	)
	if err != nil {
		return nil, err
	}

	container, err := rootClient.InspectContainer(daemon.dindContainerId)
	if err != nil {
		return nil, err
	}

	var hostAddr, hostPort string

	if endpoint.Scheme == "unix" {
		hostAddr = container.NetworkSettings.IPAddress
		hostPort = port.Port()
	} else {
		portBinding := container.NetworkSettings.Ports[port][0]
		hostAddr, _, err = net.SplitHostPort(endpoint.Host)
		if err != nil {
			return nil, err
		}
		hostPort = portBinding.HostPort
	}

	dindEndpoint := fmt.Sprintf("tcp://%v:%v", hostAddr, hostPort)
	client, err := dockerapi.NewClient(dindEndpoint)
	if err != nil {
		return nil, err
	}

	if err = backoff.Retry(client.Ping, backoff.NewExponentialBackOff()); err != nil {
		return nil, err
	}

	daemon.Client = client
	return
}

func (d *DockerDaemon) RunSimple(cmd ...string) (string, error) {
	return d.Run(dockerapi.CreateContainerOptions{
		Config: &dockerapi.Config{
			Image: "gliderlabs/alpine",
			Cmd:   cmd,
		},
	}, nil)
}

func (d *DockerDaemon) Run(createOpts dockerapi.CreateContainerOptions, startConfig *dockerapi.HostConfig) (string, error) {
	return runContainer(d.Client, createOpts, startConfig)
}

func (d *DockerDaemon) Close() error {
	if d.dindContainerId == "" {
		return nil
	}
	return d.rootClient.RemoveContainer(dockerapi.RemoveContainerOptions{
		ID:            d.dindContainerId,
		RemoveVolumes: true,
		Force:         true,
	})
}

func runContainer(client *dockerapi.Client, createOpts dockerapi.CreateContainerOptions, startConfig *dockerapi.HostConfig) (string, error) {
	err := client.PullImage(dockerapi.PullImageOptions{
		Repository: createOpts.Config.Image,
	}, dockerapi.AuthConfiguration{})
	if err != nil {
		return "", err
	}

	container, err := client.CreateContainer(createOpts)
	if err != nil {
		return "", err
	}

	err = client.StartContainer(container.ID, startConfig)
	// return container ID even if there is an error, so caller can clean up container if desired
	return container.ID, err
}

type DebugResolver struct {
	ch chan string
}

func (r *DebugResolver) AddHost(id string, addr net.IP, name string, aliases ...string) error {
	// r.ch <- fmt.Sprintf("add: %v %v %v %v", id, addr, name, aliases)
	r.ch <- fmt.Sprintf("add: %v", id)
	return nil
}

func (r *DebugResolver) RemoveHost(id string) error {
	r.ch <- fmt.Sprintf("remove: %v", id)
	return nil
}

func (r *DebugResolver) Listen() error {
	r.ch <- "listen"
	return nil
}

func (r *DebugResolver) Close() error {
	r.ch <- "close"
	return nil
}
