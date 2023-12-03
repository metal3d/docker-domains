CUR_SHA=$(shell git log -n1 --pretty='%h')
CUR_BRANCH=$(shell git branch --show-current)
VERSION=$(shell git describe --exact-match --tags $(CUR_SHA) 2>/dev/null || echo $(CUR_BRANCH)-$(CUR_SHA))
PREFIX:=/usr/local

all: build install

build: dist/docker-domains

dist/docker-domains: $(wildcard cmd/docker-domains/* dnsmasq/*)
	@[ $(shell id -u) -eq 0 ] || (echo "This script must be run as non-root." && exit 1)
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

install: build
ifneq ($(shell id -u), 0)
	@echo "Installation must be run as root. Escalating..."
	sudo make $@
else
	mkdir -p $(PREFIX)/bin
	install -m 755 dist/docker-domains $(PREFIX)/bin/docker-domains
	install -m 644 systemd/docker-domains.service /etc/systemd/system/docker-domains.service
	install -m 644 systemd/docker-domains.conf /etc/docker/docker-domains.conf
	sed -i "s|/usr/local|$(PREFIX)|g" /etc/systemd/system/docker-domains.service
	systemctl daemon-reload
	@echo
	@echo Enable service
	systemctl enable --now docker-domains
	@echo
	@echo
	@echo
	@echo "####################################################"
	@echo "# You can now start the service with:"
	@echo "#"
	@echo "# 	systemctl start docker-domains"
	@echo "#"
	@echo "#"
	@echo "# or to enable it and start it:"
	@echo "#"
	@echo "# 	systemctl enable --now docker-domains"
	@echo "#"
	@echo "#"
	@echo "# You can stop the service with:"
	@echo "#"
	@echo "# 	systemctl stop docker-domains"
	@echo "#"
	@echo "#"
	@echo "# You can enable the service to start at boot with:"
	@echo "#"
	@echo "# 	systemctl enable docker-domains"
	@echo "#"
	@echo "#####################################################"
endif
	
uninstall:
ifneq ($(shell id -u), 0)
	echo "Uninstallation must be run as root. Escalating..."
	sudo make $@
else
# uninstall the binary
	rm -f $(PREFIX)/bin/docker-domains
	# uninstall systemd service
	systemctl disable --now docker-domains || true
	rm -f /etc/systemd/system/docker-domains.service
	systemctl daemon-reload
endif

clean:
	rm -rf dist .cache
