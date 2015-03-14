package dockerpool

import (
	"net/url"
	"os"
	"path/filepath"

	"github.com/joeshaw/multierror"

	dockerapi "github.com/fsouza/go-dockerclient"
)

type DockerDaemon struct {
	Client *dockerapi.Client
	Close  func() error
}

func getopt(name, def string) string {
	if env := os.Getenv(name); env != "" {
		return env
	}
	return def
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

func NewNativeDockerDaemon() (*DockerDaemon, error) {
	client, _, err := clientFromEnv()
	if err != nil {
		return nil, err
	}

	daemon := &DockerDaemon{Client: client}
	daemon.Close = daemon.KillAllContainers
	return daemon, nil
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

func (d *DockerDaemon) KillAllContainers() error {
	containers, err := d.Client.ListContainers(dockerapi.ListContainersOptions{})
	if err != nil {
		return err
	}

	var errs multierror.Errors

	for _, container := range containers {
		err = d.Client.KillContainer(dockerapi.KillContainerOptions{
			ID: container.ID,
		})
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs.Err()
}

func runContainer(client *dockerapi.Client, createOpts dockerapi.CreateContainerOptions, startConfig *dockerapi.HostConfig) (string, error) {
	container, err := client.CreateContainer(createOpts)
	if err != nil {
		return "", err
	}

	err = client.StartContainer(container.ID, startConfig)
	// return container ID even if there is an error, so caller can clean up container if desired
	return container.ID, err
}
