package dnsmasq

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// Prepare resolved to use dnsmasq. It creates a dnsmasq.conf file
// in /etc/systemd/resolved.conf.d/docker-dnsmasq.conf and add the DNS entry
// that points on the docker0 interface.
// It also creates a temporary directory where the dnsmasq configuration will
// be written by docker-domains.
func ConfigureSystemdResolved() {
	// get IP of docker0 interface
	ipaddr := getDockerIP()

	// configure resolved to use dnsmasq
	// ensure that the directory exists
	dirname := filepath.Dir(resolveDnsmasq)
	if _, err := os.Stat(dirname); err != nil {
		if err := os.Mkdir(dirname, 0755); err != nil {
			log.Fatal(err)
		}
	}
	// and create the configuration
	fp, err := os.Create(resolveDnsmasq)
	if err != nil {
		log.Fatal(err)
	}

	fp.WriteString("[Resolve]\n")
	fp.WriteString("DNS=" + ipaddr + "\n")
	fp.Close()

	// create domain file
	fp, err = os.Create(TempDir + "/dnsmasq.conf")
	if err != nil {
		log.Fatal(err)
	}
	fp.WriteString(DNSMasqHeaderConfig)
	fp.Close()

	// set it readable
	err = os.Chmod(TempDir+"/dnsmasq.conf", 0644)
	if err != nil {
		log.Fatal(err)
	}

	RestoreCon()
}

// For SELinux systems, we need to restore the context of the created files.
func RestoreCon() {
	// if SELinux, restore the context
	err := exec.Command("selinuxenabled").Run()
	if err == nil {
		dirname := filepath.Dir(resolveDnsmasq)
		log.Println("Restoring SELinux context")
		err = exec.Command("restorecon", "-R", dirname).Run()
		if err != nil {
			log.Println("Failed to restore SELinux context on", dirname)
		}

		err = exec.Command("restorecon", "-R", TempDir).Run()
		if err != nil {
			log.Println("Failed to restore SELinux context on", TempDir)
		}
	}
}

// Remove the dnsmasq configuration file
func UnconfigureSystemdResolved() {
	log.Println("Removing configured dnsmasq from resolved", resolveDnsmasq)
	// remove dnsmasq configuration
	os.Remove(resolveDnsmasq)

	// reload systemd-resolved
	ReloadResolved()
}

// RefreshCache
func RefreshCache() {
	// systemd-resolve --flush-caches
	log.Println("Refreshing resolved cache")
	err := exec.Command("systemd-resolve", "--flush-caches").Run()
	if err != nil {
		log.Println("Failed to refresh resolved cache", err)
	}
}

func ReloadResolved() {
	// reload systemd-resolved
	log.Println("Reloading systemd-resolved")
	err := exec.Command("systemctl", "condrestart", "systemd-resolved.service").Run()
	if err != nil {
		log.Println("Failed to reload systemd-resolved", err)
	}
}
