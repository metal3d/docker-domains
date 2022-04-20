# Docker Domain resolution

This daemon allows you to assign domain names (with wildcard support) to your Docker containers that are resolved locally and within the containers themselves.

It uses:
- dnsmasq
- your systemd-resolved configuration

# The principle

The daemon will simply start a dnsmasq service listening on your `docker0` interface. It will then tell systemd-resolved that this interface should be used to resolve domain names (while continuing to resolve external domain names).

Whenever a container starts or is stopped:
- the daemon reads the list of Docker containers
- it creates a domain name `container_name.docker` (you can change this in `/etc/docker/docker-domains.conf`)
- it creates a domain name `container_name.network_name.docker` if the container is started inside a Docker network
- if the container has a hostname, it will also be added

Keep in mind that `.docker` can be changed in `/etc/docker/docker-domains.conf`.

Subdomains are also resolved. So if your container is started with a hostname at "foo.docker", the addresses `*.foo.docker` will point to container.

All domains are also resolved **inside** containers! That's the reason this project exists.

> You theorically not need to change firewall configuration. If you have any problem and/or solution, please fill an issue and/or provide a pull-request.

# Installation

Clone this repository, then in a terminal, go to the directory and type:

```bash
make build
sudo make install
```

To activate the service:

```bash
sudo systemctl start docker-domains
```
You can also start it with your system:

```bash
# --now indicates to start the service at the same time
sudo systemctl enable --now docker-domains
```

You can uninstall with the command:

```bash
sudo make uninstall
```

The compilation is done with a Docker container, so you don't need to install the `go` compiler on your computer.

# What happens when you shut down the service?

When `docker-domain` is shut down, the daemon-specific configuration in `systemd-resolved` is removed and `dnsmasq` is stopped. Thus, you are back to the original situation.

