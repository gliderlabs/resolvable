package dockerpool

import (
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/cenkalti/backoff"
	dockerapi "github.com/fsouza/go-dockerclient"
)

type DockerInDocker struct {
	client      *dockerapi.Client
	containerId string
}

type ClientInit func(*dockerapi.Client) error

func NewDockerInDockerDaemon() (*DockerDaemon, error) {
	rootClient, endpoint, err := clientFromEnv()
	if err != nil {
		return nil, err
	}
	return newDockerInDockerDaemon(rootClient, endpoint, pingClient)
}

var pingClient ClientInit = func(client *dockerapi.Client) error {
	return client.Ping()
}

func newDockerInDockerDaemon(rootClient *dockerapi.Client, endpoint *url.URL, clientInit ClientInit) (*DockerDaemon, error) {
	var err error

	dockerInDocker := &DockerInDocker{
		client: rootClient,
	}
	daemon := &DockerDaemon{
		Close: dockerInDocker.Close,
	}
	defer func() {
		// if there is an error, client will not be set, so clean up
		if daemon.Client == nil {
			daemon.Close()
		}
	}()

	port := dockerapi.Port("4444/tcp")

	dockerInDocker.containerId, err = runContainer(rootClient,
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

	container, err := rootClient.InspectContainer(dockerInDocker.containerId)
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

	b := backoff.NewExponentialBackOff()
	// retry a bit faster than the defaults
	b.InitialInterval = time.Second / 10
	b.Multiplier = 1.1
	b.RandomizationFactor = 0.2
	// don't need to wait a full minute to timeout
	b.MaxElapsedTime = 30 * time.Second

	if err = backoff.Retry(func() error { return clientInit(client) }, b); err != nil {
		return nil, err
	}

	daemon.Client = client
	return daemon, err
}

func (d *DockerInDocker) Close() error {
	if d.containerId == "" {
		return nil
	}
	return d.client.RemoveContainer(dockerapi.RemoveContainerOptions{
		ID:            d.containerId,
		RemoveVolumes: true,
		Force:         true,
	})
}
