package client

import (
	"fmt"
	"github.com/fmnx/cftun/log"
	tunToArgo "github.com/xjasonlyu/tun2socks/v2/engine"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type Tun struct {
	Enable    bool     `yaml:"enable" json:"enable"`
	Name      string   `yaml:"name" json:"name"`
	Interface string   `yaml:"interface" json:"interface"`
	LogLevel  string   `yaml:"log-level" json:"log-level"`
	Routes    []string `yaml:"routes" json:"routes"`
}

func (t *Tun) Run(key *tunToArgo.Key) {

	tunToArgo.Insert(key)
	go tunToArgo.Start()

	switch runtime.GOOS {
	case "linux":
		t.LinuxConfigure()
	case "windows":
		go func() {
			time.Sleep(1 * time.Second)
			t.WindowsConfigure()
		}()
	case "darwin":
		go func() {
			time.Sleep(1 * time.Second)
			t.DarwinConfigure()
		}()
	}
}

func (t *Tun) LinuxConfigure() {
	if err := exec.Command("ip", "tuntap", "add", "mode", "tun", "dev", t.Name).Run(); err != nil {
		log.Errorln("failed to add tun %s: %w", t.Name, err)
	}

	if err := exec.Command("ip", "addr", "add", "198.18.0.1/15", "dev", t.Name).Run(); err != nil {
		log.Errorln("failed to add IPv4 address to %s: %w", t.Name, err)
	}

	if err := exec.Command("ip", "link", "set", t.Name, "up").Run(); err != nil {
		log.Errorln("failed to set %s up: %w", t.Name, err)
	}

	for _, route := range t.Routes {
		if strings.Contains(route, ":") {
			_ = exec.Command("ip", "-6", "route", "add", route, "dev", t.Name).Run()
			continue
		}
		_ = exec.Command("ip", "route", "add", route, "dev", t.Name).Run()
	}
}

func (t *Tun) WindowsConfigure() {
	// netsh interface ipv4 set address name="wintun" source=static addr=192.168.123.1 mask=255.255.255.0
	if err := exec.Command("netsh", "interface", "ipv4", "set", "address", t.Name, "static", "192.168.123.1/24").Run(); err != nil {
		log.Errorln("failed to add ipv4 address for tun device %s: %w", t.Name, err)
	}

	// netsh interface ipv6 add address "tun0" fd12:3456:789a::1/64
	if err := exec.Command("netsh", "interface", "ipv6", "add", "address", t.Name, "fd12:3456:789a::1/64").Run(); err != nil {
		log.Errorln("failed to add ipv6 address for tun device %s: %w", t.Name, err)
	}

	// netsh interface ipv4 set dnsservers name="wintun" static address=8.8.8.8 register=none validate=no
	if err := exec.Command("netsh", "interface", "ipv4", "set", "dnsservers", fmt.Sprintf("name=%s", t.Name),
		"static", "address=8.8.8.8", "register=none", "validate=no").Run(); err != nil {
		log.Errorln("failed to set dns server for tun %s: %w", t.Name, err)
	}

	for _, route := range t.Routes {
		if strings.Contains(route, ":") {
			if !strings.Contains(route, "/") {
				route += "/128"
			}
			_ = exec.Command("netsh", "interface", "ipv6", "add", "route", route, t.Name, "fd12:3456:789a::1", "metric=1").Run()
			continue
		}
		if !strings.Contains(route, "/") {
			route += "/32"
		}
		_ = exec.Command("netsh", "interface", "ipv4", "add", "route", route, t.Name, "192.168.123.1", "metric=1").Run()
	}
}

func (t *Tun) DarwinConfigure() {
	// sudo ifconfig utun123 198.18.0.1 198.18.0.1 up
	if err := exec.Command("sudo", "ifconfig", t.Name, "198.18.0.1", "198.18.0.1", "up").Run(); err != nil {
		log.Errorln("failed to add ipv4 address for tun %s: %w", t.Name, err)
	}

	// sudo ifconfig utun123 inet6 fd12:3456:789a::1/64 up
	if err := exec.Command("sudo", "ifconfig", t.Name, "inet6", "fd12:3456:789a::1/64", "up").Run(); err != nil {
		log.Errorln("failed to add ipv6 address for tun %s: %w", t.Name, err)
	}

	for _, route := range t.Routes {
		// sudo route -inet6 add -net 2001:db8::/32 2001:db8:1::1
		if strings.Contains(route, ":") {
			if !strings.Contains(route, "/") {
				route += "/128"
			}
			_ = exec.Command("sudo", "route", "-inet6", "add", "-net", route, "fd12:3456:789a::1").Run()
			continue
		}
		// sudo route add -net 1.0.0.0/8 198.18.0.1
		if strings.Contains(route, "/") {
			route += "/32"
		}
		_ = exec.Command("sudo", "route", "add", "-net", route, "198.18.0.1").Run()
	}
}

func DeleteTunDevice(tunName string) {
	tunToArgo.Stop()
	if runtime.GOOS != "linux" {
		return
	}
	_ = exec.Command("ip", "link", "set", tunName, "down").Run()
	_ = exec.Command("ip", "tuntap", "del", tunName, "mode", "tun").Run()
}
