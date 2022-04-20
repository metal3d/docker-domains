package main

import (
	"context"
	"docker-domains/dnsmasq"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

var (
	version               = "master"
	globalLock            = sync.Mutex{}
	dockerDomainExtension = os.Getenv("DOCKER_DOMAIN")
	defaultNetworkName    = os.Getenv("DOCKER_DEFAULT_NETWORK")
	specificNames         = os.Getenv("DOCKER_STATIC_NAMES")
)

// This build a dnsmasq config for container.
func buildDNSMasqConfig(container types.Container, cli *client.Client) string {
	// get the network ip, container name and possible given hostname
	ip := ""
	hosts := []string{}
	if defaultNetworkName == "" {
		defaultNetworkName = "bridge"
	}
	networkName := ""
	for name, network := range container.NetworkSettings.Networks {
		ip = network.IPAddress
		if name != defaultNetworkName {
			networkName = name
		}
	}

	name := container.Names[0][1:]
	inspect, err := cli.ContainerInspect(context.Background(), container.ID)
	if err != nil {
		fmt.Println(err)
	} else {
		domain := inspect.Config.Domainname
		hostname := inspect.Config.Hostname
		if hostname != "" {
			if domain != "" {
				hostname = hostname + "." + domain
			}
			hosts = append(hosts, "address=/."+hostname+"/"+ip)
		}
	}

	// default to the container name.(from config)
	if dockerDomainExtension != "" && !strings.HasPrefix(dockerDomainExtension, ".") {
		dockerDomainExtension = "." + dockerDomainExtension
	}

	// by default, we add the container name with .networkname.tld
	// to the hosts file.
	if networkName != "" {
		hosts = append(hosts, "address=/."+name+"."+networkName+dockerDomainExtension+"/"+ip)
	} else {
		hosts = append(hosts, "address=/."+name+dockerDomainExtension+"/"+ip)
	}

	// specificNames is a comma separated list of hostnames to add to the hosts file.
	for _, host := range strings.Split(specificNames, ",") {
		block := strings.Split(host, ":")
		if len(block) == 2 && block[0] == name {
			hosts = append(hosts, "address=/."+block[1]+"/"+ip)
		}
	}

	// add the ip address to the config
	hostname := strings.Join(hosts, "\n") + "\n"
	return hostname

}

// removes the config files.
func cleanDnsConfFiles() {
	if dnsmasq.TempDir != "/tmp" {
		log.Println("Removing dnsmasq config files")
		os.RemoveAll(dnsmasq.TempDir)
	}
}

// Called when TERM/INT Signal is received. We removes dnsmasq config files, systemd-resolved config and we reload it to be back to original config.
func onStop() {
	log.Println("Terminating...")
	dnsmasq.Stop() // stop dnsmasq forever
	cleanDnsConfFiles()
	dnsmasq.UnconfigureSystemdResolved()
	log.Println("Terminated")
}

// write the config to the temp dir. This function get only the "running" containers.
// As it can be called a bit too fast, containers can be "running" while there actually stopping. So we
// need to time.Sleep a bit.
// After the configuration is created, we refresh the systemd-resolved cache.
func writeConfig(cli *client.Client) {
	globalLock.Lock()
	defer globalLock.Unlock()
	defer dnsmasq.RefreshCache()

	time.Sleep(200 * time.Millisecond)

	// list all containers
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{
		Filters: filters.NewArgs(filters.Arg("status", "running")),
	})
	if err != nil {
		log.Fatal(err)
	}
	dnsmasqfile := dnsmasq.TempDir + "/dnsmasq.conf"
	fp, err := os.Create(dnsmasqfile)
	if err != nil {
		log.Fatal(err)
	}
	defer fp.Close()
	fp.WriteString(dnsmasq.DNSMasqHeaderConfig)
	for _, container := range containers {
		fmt.Println("========>", container.Names[0], container.State)
		// get the ip address
		config := buildDNSMasqConfig(container, cli)
		// write the config to a file
		if err != nil {
			log.Println(err)
			continue
		}
		fp.WriteString(config)
	}
}

func main() {

	showVersion := false
	flag.BoolVar(&showVersion, "version", showVersion, "show the current version")
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		return
	}

	// Prepare dnsmasq and systemd-resolved config.
	dnsmasq.Initialize()
	dnsmasq.ConfigureSystemdResolved()

	// Connect to Docker API.
	cli, err := client.NewEnvClient()
	if err != nil {
		log.Println(err)
	}

	// write the config to the temp dir for the first time
	// to manage already started containers
	writeConfig(cli)

	// now we can start dnsmasq
	dnsmasq.Start()

	// We need to be sure that the dnsmasq is stopped when the service is killed or stopped.
	sig := make(chan os.Signal, 0)
	stopEvent := make(chan int, 0)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	go func() {
		s := <-sig // got signal
		fmt.Println("Got signal:", s)
		onStop()       // remove dnsmasq config files and systemd-resolved config
		stopEvent <- 1 // stop the event loop (below)
	}()

	// listen for events
	events, eventError := cli.Events(context.Background(), types.EventsOptions{
		Filters: filters.NewArgs(
			filters.Arg("type", "container"),
			filters.Arg("event", "start"),
			filters.Arg("event", "stop"),
			filters.Arg("event", "die"),
		),
	})

	// When a filtered event is received, we write the config again. This will
	// trigger a restart of dnsmasq + a refresh of the systemd-resolved cache.
	for {
		select {
		case <-stopEvent:
			cli.Close()
			os.Exit(0)
		case <-events:
			// do something with the event
			writeConfig(cli)
		case err := <-eventError:
			// handle error
			log.Println(err)
		}
	}
}
