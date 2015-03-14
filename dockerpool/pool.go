package dockerpool

import (
	"bytes"
	"net/url"

	dockerapi "github.com/fsouza/go-dockerclient"

	"runtime"
)

type Pool interface {
	Borrow() (*DockerDaemon, error)
	Return(*DockerDaemon)
	Close()
}

type BasePool struct {
	pool chan *DockerDaemon
}

func (p *BasePool) Return(d *DockerDaemon) {
	select {
	case p.pool <- d:
	default:
		d.Close()
	}
}

func (p *BasePool) Close() {
	close(p.pool)
	for d := range p.pool {
		d.Close()
	}
}

type NativePool struct {
	BasePool
}

func NewNativePool(preloadImages ...string) (*NativePool, error) {
	daemon, err := NewNativeDockerDaemon()
	if err != nil {
		return nil, err
	}

	if err = pullImages(daemon.Client, preloadImages); err != nil {
		return nil, err
	}

	pool := &NativePool{BasePool{make(chan *DockerDaemon, 1)}}
	pool.Return(daemon)
	return pool, nil
}

func (p *NativePool) Borrow() (*DockerDaemon, error) {
	d := <-p.pool
	d.KillAllContainers()
	return d, nil
}

type DockerInDockerPool struct {
	BasePool
	client        *dockerapi.Client
	endpoint      *url.URL
	preloadImages [][]byte
}

func NewDockerInDockerPool(preloadImages ...string) (*DockerInDockerPool, error) {
	client, endpoint, err := clientFromEnv()
	if err != nil {
		return nil, err
	}

	if err = pullImages(client, append(preloadImages, "jpetazzo/dind")); err != nil {
		return nil, err
	}

	imageData := make([][]byte, 0, len(preloadImages))
	var buf bytes.Buffer

	for _, image := range preloadImages {
		err = client.ExportImage(dockerapi.ExportImageOptions{
			Name:         image,
			OutputStream: &buf,
		})
		if err != nil {
			return nil, err
		}
		imageData = append(imageData, buf.Bytes())
		buf.Reset()
	}

	pool := make(chan *DockerDaemon, runtime.GOMAXPROCS(-1))
	return &DockerInDockerPool{BasePool{pool}, client, endpoint, imageData}, nil
}

func (p *DockerInDockerPool) clientInit(client *dockerapi.Client) error {
	if len(p.preloadImages) == 0 {
		return client.Ping()
	}

	for _, imageData := range p.preloadImages {
		err := client.LoadImage(dockerapi.LoadImageOptions{
			InputStream: bytes.NewReader(imageData),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *DockerInDockerPool) Borrow() (*DockerDaemon, error) {
	select {
	case d := <-p.pool:
		d.KillAllContainers()
		return d, nil
	default:
		return newDockerInDockerDaemon(p.client, p.endpoint, p.clientInit)
	}
}

func pullImages(client *dockerapi.Client, images []string) error {
	for _, image := range images {
		err := client.PullImage(dockerapi.PullImageOptions{
			Repository: image,
		}, dockerapi.AuthConfiguration{})
		if err != nil {
			return err
		}
	}

	return nil
}
