# Docker Domain resolution

This daemon allows you to assign domain names (with wildcard support) to your Docker containers that are resolved locally and within the containers themselves.

It uses:
- dnsmasq
- your systemd-resolved configuration

# Table of contents

- [Docker Domain resolution](#docker-domain-resolution)
- [The principle](#the-principle)
- [Installation](#installation)
- [Configuration](#configuration)
- [Examples of use](#examples-of-use)
  - [Simple tests](#simple-tests)
  - [Kind](#kind)
  - [What if I want SSL/TLS?](#what-if-i-want-ssltls)
  - [Wildcard example with SSL/TLS](#wildcard-example-with-ssltls)
- [What happens when you shut down the service?](#what-happens-when-you-shut-down-the-service)
- [What about Traefik](#what-about-traefik)

# The principle

The daemon will simply start a dnsmasq service listening on your `docker0` interface. It will then tell systemd-resolved that this interface should be used to resolve domain names (while continuing to resolve external domain names).

Whenever a container starts or is stopped:
- the daemon reads the list of Docker containers
- it creates a domain name `container_name.docker` (you can change this in `/etc/docker/docker-domains.conf`)
- it creates a domain name `container_name.network_name.docker` if the container is started inside a Docker network
- if the container has a hostname, it will also be added
- if container has got a domain, it will append the domain to hostname
- if you specified `DOCKER_STATIC_NAMES` in confguration and the container name corresponds, so the corresponding domain name is also added (interessing with [Kind](https://kind.sig.k8s.io) with [`ingress-nginx`](https://artifacthub.io/packages/helm/ingress-nginx/ingress-nginx) with `controller.hostPort` to "true")

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


# Configuration

The configuration is placed in `/etc/docker/docker-domains.conf` - you must restart the service after changes.

This is what you can set:

- `DOCKER_DOMAIN` this is the "domain" to add to container name. E.g. a container named "foo" will respond to "foo.docker" if `DOCKER_DOMAIN` is set to `.docker`
- `DOCKER_INTERFACE` is the docker net interface. It's commonly `docker0`, don't change it unless you know what you do
- `DOCKER_DEFAULT_NETWORK` is the default network for you containers unless you specify one. This is commonly `bridge`, and `docker-domains` will not append the network name in domain name for this network
- `DOCKER_STATIC_NAMES` is a coma spearated list of `name:domain` to force. It's pretty interessing when you use a software which starts contiainers without letting you the choice of container name. E.g. for [Kind](https://kind.sigs.k8s.io) names the control plane `kind_control_plane`. If you use a Ingress Controller that bind ports on this container, so you can propose a domamin name to access your web applications. **Don't forget to add a dot to the domain name to allow wildcard** 

# Examples of use

## Simple tests

After having installed and started the `docker-domains` service:

```bash
# no need to bind ports :)
docker run --rm --name website --hostname foo.com -d nginx

# Try to get pages:
curl foo.com
curl website.docker

# stop
docker stop website

# then resolution should fail
curl website.docker
```

## Kind

[Kind](https://kind.sigs.k8s.io) is a local Kubernetes instance that will start in Docker. The default master is named `kind_control_plane`. You can start a Ingress Controller inside the Kubernetes instance to hit your web applications with a local domain.

```bash
# setup DOCKER_STATIC_NAMES=kind_control_plane:.kind
# and do "sudo systemctl restart docker-domains"

# create a cluster
kind create cluster

# wait for started, type "kubectl get nodes" - node should be "Ready" after a few seconds

# add an ingress controller listening on 8à/443 ports on control plane
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm install -n ingress ingress-controller ingress-nginx/ingress-nginx \
    --create-namespace \
    --set controller.hostPort=true

# check "kubectl -n ingress get pods", all pods should be "Running" after a few seconds

# Now create a "ghost" website for example, set the "hostname" to "something.kind"
# e.g. ghost.kind
helm repo add bitnami https://charts.bitnami.com/bitnami
helm install -n ghost ghost bitnami/ghost --create-namespace \
    --set ingress.enabled=true \
    --set ingress.ingressClassName=nginx \
    --set ingress.hostname=ghost.kind

```

After a few seconds, you can go to http://ghost.kind

It works because:
- `kind_control_plane` IP address is resolved for **all** `.kind` subdomains (applied in configuration)
- the ingress controller is configured to listen on ports 80 and 443 on "all nodes", so on `kind_control_plane` also
- the ingress controller routes the subdomains to the corresponding "Pods"

It's a bit easier than using `nip.io` with the risk of a IP change across reboot.



## What if I want SSL/TLS?

You need to add your own reverse proxy if you're not using a service that can handle certificates. But, in short, it's easy to do with `mkcert`.

Install certutil and mkcert:

```bash
sudo apt install libnss3-tools
    -or-
sudo dnf install nss-tools
    -or-
sudo pacman -S nss
    -or-
sudo zypper install mozilla-nss-tools

curl -L https://github.com/FiloSottile/mkcert/releases/download/v1.4.3/mkcert-v1.4.3-linux-amd64 -o ~/.local/bin/mkcert
chmod +x ~/.local/bin/mkcert
```


E.g. with Nginx.

First, create a certificate and key inside a local directory of your project:
```bash
mkdir -p nginx/{conf.d,certs}

# Do it only once, install CA root to your trust store (chrome/chromium/brave, firefox...) 
mkcert -install

# create foo.com certificate and key
mkcert --install --cert-file nginx/certs/foo.com.crt --key-file nginx/certs/foo.com.key foo.com
```

Then, create the `nginx/conf.d/default.conf` changing `YOUR_CONTAINER` to a service/container name (avoid domain):

```
server {
    listen :443;

    ssl_certificate     /etc/nginx/certs/foo.com.crt;
    ssl_certificate_key /etc/nginx/certs/foo.com.key;

    location / {
        proxy_pass http://YOUR_CONTAINER
    }
}
```

In youd docker-compose file, add this:

```yaml
services:
  #...
  reverseproxy:
    image: nginx:alpine
    volumes:
    - nginx/conf.d:/etc/nginx/conf.d:z
    - nginx/certs:/etc/nginx/certs:z
    hostname: foo.com
    depends_on:
    - YOUR_CONTAINER
```

And that's all. `foo.com` can now be resolved and you can connect https://foo.com

## Wildcard example with SSL/TLS

We can use the example above to imaginate a project that has got several web applications.

Set the certificat to wildcard:

```bash
mkcert -cert-file nginx/certs/foo.com.crt -key-file nginx/certs/foo.com.key "*.foo.com"
```

Change the nginx configuration to server 2 subdomains:
```

server {
    listen :443;
    server_name site1.foo.com; # first subdomain

    ssl_certificate     /etc/nginx/certs/foo.com.crt;
    ssl_certificate_key /etc/nginx/certs/foo.com.key;

    location / {
        # point on a first container
        proxy_pass http://YOUR_CONTAINER_1
    }
}
server {
    listen :443;
    server_name site2.foo.com; # second subdomain

    ssl_certificate     /etc/nginx/certs/foo.com.crt;
    ssl_certificate_key /etc/nginx/certs/foo.com.key;

    location / {
        # point on another container
        proxy_pass http://YOUR_CONTAINER_2
    }
}
```

Because docker-domains configure dnsmasq to resolve all subdomains of "foo.com", so both site1.foo.com and site2.foo.com will be resolved to the Nginx container. Nginx will detect the requested subdomain to route clients to the right container.

# What happens when you shut down the service?

When `docker-domain` is shut down, the daemon-specific configuration in `systemd-resolved` is removed and `dnsmasq` is stopped. Thus, you are back to the original situation.

# What about Traefik

Traefik is nice. Really, I like it.

But there are drawbacks:

- it's a reverse proxy, you will need to use `.localhost` domains or touch yourself `/etc/hosts` to resolve others domain names. So, to reproduce a customer settings in production, it's a bit complexe
- with standard and respectful distrubution (so, not Ubuntu...), you cannot bind ports 80 and 443 with a container started as standar user. That means that Traefik should be launched as root
- with standard and... ok... `/etc/hosts` cannot be used to respond to "wildcard" domains. That means that you will need to add all your subdomains in `/etc/hosts`
- you need to remember to clean up `/etc/hosts` file...
- Traefik can be complicated with dynamic Varnish activation - some users reports me this, I don't have details
- As it's a reverse proxy, it may break or add HTTP Headers

Using Domain resolution is lightweight and doesn't add a layer. Also, you can use whatever the protocol you want, **it's not only for HTTP**.
