# Resolvable - Docker DNS resolver

A simple DNS server to resolve names of local Docker containers.

`Added by Schinti95:` IP Address lookup when using docker networks.

`resolvable` is intended to run in a Docker container:

	docker run -d \
		--hostname resolvable \
		-v /var/run/docker.sock:/tmp/docker.sock \
		-v /etc/resolv.conf:/tmp/resolv.conf \
		mgood/resolvable

The `docker.sock` is mounted to allow `resolvable` to listen for Docker events and automatically register containers.

`resolvable` can insert itself into the host's `/etc/resolv.conf` file by mounting this file to `/tmp/resolv.conf` in the container. When starting, it will insert itself as the first `nameserver` in the file, and remove itself when shutting down.

## Systemd integration

On systems using systemd, `resolvable` can integrate with the systemd DNS configuration. Instead of mounting `/etc/resolv.conf`, mount the systemd configuration path `/run/systemd` and the DBUS socket as follows:

	docker run -d \
		--hostname resolvable \
		-v /var/run/docker.sock:/tmp/docker.sock \
		-v /run/systemd:/tmp/systemd \
		-v /var/run/dbus/system_bus_socket:/var/run/dbus/system_bus_socket \
		mgood/resolvable

`resolvable` will generate a systemd network config, and then use the DBUS socket to reload `systemd-networkd` to regenerate the host's `/etc/resolv.conf`.

## Container Registration

`resolvable` provides DNS entries `<hostname>` and `<name>.docker` for each container. Containers are automatically registered when they start, and removed when they die.

For example, the following container would be available via DNS as `myhost` and `myname.docker`:

	docker run -d \
		--hostname myhost \
		--name myname \
		mycontainer

## DNS Forwarding

`resolvable` also supports forwarding DNS queries to other containers providing DNS servers. This integrates well with tools like Consul or SkyDNS that offer a DNS endpoint for service discovery.

Containers configured with the `DNS_RESOLVES` environment variable are registered in `resolvable` to forward DNS queries for any domains listed.

To run an example `consul` container, supporting DNS queries for the `.consul` domain on port `8600`:

	docker run -d \
		-e DNS_RESOLVES=consul \
		-e DNS_PORT=8600 \
		-p 8600/udp \
		consul

`DNS_RESOLVES` must contain least one domain to forward to this container. Multiple values can be provided as a comma-separated list.

`DNS_PORT` is optional, and defaults to `53`.

## Interface Addresses

`resolvable` also provides a DNS entry for the Docker bridge interface address, usually `docker0`. This can be used to communicate with services with a known port bound to the Docker bridge.

See this article on [Docker network configuration](https://docs.docker.com/articles/networking/) for additional details on the Docker bridge interface.

<img src="https://ga-beacon.appspot.com/UA-58928488-2/resolvable/readme?pixel" />
