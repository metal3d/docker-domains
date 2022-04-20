CUR_SHA=$(shell git log -n1 --pretty='%h')
CUR_BRANCH=$(shell git branch --show-current)
VERSION=$(shell git describe --exact-match --tags $(CUR_SHA) 2>/dev/null || echo $(CUR_BRANCH)-$(CUR_SHA))
PREFIX:=/usr/local

build: dist/docker-domains

dist/docker-domains: $(wildcard cmd/docker-domains/* dnsmasq/*)
	# build the Go project with docker
	mkdir -p .cache
	# build
	docker run --rm -e HOME=/tmp \
		--user $(shell id -u):$(shell id -g) \
		-v $(PWD)/.cache:/tmp/.cache:z -v $(PWD):/go/src/docker-domains:z \
		-w /go/src/docker-domains \
		golang:1.18  \
		go build -ldflags="-X 'main.version=$(VERSION)'" -o dist/docker-domains ./cmd/docker-domains
	# strip the binary
	strip dist/docker-domains
	# ensure it's executable
	chmod +x dist/docker-domains

install:
	@[ -f dist/docker-domains ] || (echo "Please run 'make build' (without sudo) first" && exit 1)
	@[ $(shell id -u) -eq 0 ] || (echo "This script must be run as root" && exit 1)
	mkdir -p $(PREFIX)/bin
	install -m 755 dist/docker-domains $(PREFIX)/bin/docker-domains
	install -m 644 systemd/docker-domains.service /etc/systemd/system/docker-domains.service
	sed -i "s|/usr/local|$(PREFIX)|g" /etc/systemd/system/docker-domains.service
	systemctl daemon-reload
	#
	# You can now start the service with:
	# systemctl start docker-domains
	# or to enable it and start it:
	# systemctl enable --now docker-domains
	# You can stop the service with:
	# systemctl stop docker-domains
	# You can enable the service to start at boot with:
	# systemctl enable docker-domains
	
uninstall:
	# ensire it's sudo or root
	if [ $(shell id -u) -ne 0 ]; then \
		echo "This script must be run as root"; \
		exit 1; \
	fi
	# uninstall the binary
	rm -f $(PREFIX)/bin/docker-domains
	# uninstall systemd service
	systemctl disable --now docker-domains || true
	rm -f /etc/systemd/system/docker-domains.service
	systemctl daemon-reload

clean:
	rm -rf dist .cache
