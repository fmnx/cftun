package client

import (
	"fmt"
	tunToArgo "github.com/fmnx/cftun/client/tun2argo/engine"
	"github.com/fmnx/cftun/client/tun2argo/proxy"
	"github.com/fmnx/cftun/log"
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
	Ipv4      string   `yaml:"ipv4" json:"ipv4"`
	Ipv6      string   `yaml:"ipv6" json:"ipv6"`
	MTU       int      `yaml:"mtu" json:"mtu"`
}

func (t *Tun) ipv4() string {
	if t.Ipv4 != "" {
		return t.Ipv4
	}
	switch runtime.GOOS {
	case "windows":
		return "192.168.123.1"
	case "darwin":
		return "192.168.123.1"
	default:
		return "198.18.0.1"
	}
}

func (t *Tun) ipv6() string {
	if t.Ipv6 != "" {
		return t.Ipv6
	}
	return "fd12:3456:789a::1"
}

func (t *Tun) mtu() int {
	if t.MTU == 0 {
		return 1280
	}
	return t.MTU

}

func (t *Tun) Run(scheme, cdnIP, url string, port int) {

	argoProxy := proxy.NewArgo(scheme, cdnIP, url, port)
	err := tunToArgo.HandleNetStack(argoProxy, t.Name, t.Interface, t.LogLevel, t.mtu())
	if err != nil {
		log.Fatalln(err.Error())
	}

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
		//log.Errorln("failed to add tun %s: %s", t.Name, err.Error())
	}

	if err := exec.Command("ip", "addr", "add", t.ipv4(), "dev", t.Name).Run(); err != nil {
		log.Errorln("failed to add IPv4 address to %s: %w", t.Name, err)
	}

	if err := exec.Command("ip", "-6", "addr", "add", t.ipv6(), "dev", t.Name).Run(); err != nil {
		log.Errorln("failed to add IPv6 address to %s: %w", t.Name, err)
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
	if err := exec.Command("netsh", "interface", "ipv4", "set", "address", t.Name, "static", t.ipv4()).Run(); err != nil {
		log.Errorln("failed to add ipv4 address for tun device %s: %w", t.Name, err)
	}

	// netsh interface ipv6 add address "tun0" fd12:3456:789a::1/64
	if err := exec.Command("netsh", "interface", "ipv6", "add", "address", t.Name, t.ipv6()).Run(); err != nil {
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
			_ = exec.Command("netsh", "interface", "ipv6", "add", "route", route, t.Name, t.ipv6(), "metric=1").Run()
			continue
		}
		if !strings.Contains(route, "/") {
			route += "/32"
		}
		_ = exec.Command("netsh", "interface", "ipv4", "add", "route", route, t.Name, t.ipv4(), "metric=1").Run()
	}
}

func (t *Tun) DarwinConfigure() {
	// sudo ifconfig utun123 198.18.0.1 198.18.0.1 up
	if err := exec.Command("sudo", "ifconfig", t.Name, t.ipv4(), t.ipv4(), "up").Run(); err != nil {
		log.Errorln("failed to add ipv4 address for tun %s: %w", t.Name, err)
	}

	// sudo ifconfig utun123 inet6 fd12:3456:789a::1/64 up
	if err := exec.Command("sudo", "ifconfig", t.Name, "inet6", t.ipv6(), "up").Run(); err != nil {
		log.Errorln("failed to add ipv6 address for tun %s: %w", t.Name, err)
	}

	for _, route := range t.Routes {
		// sudo route -inet6 add -net 2001:db8::/32 2001:db8:1::1
		if strings.Contains(route, ":") {
			if !strings.Contains(route, "/") {
				route += "/128"
			}
			_ = exec.Command("sudo", "route", "-inet6", "add", "-net", route, t.ipv6()).Run()
			continue
		}
		// sudo route add -net 1.0.0.0/8 198.18.0.1
		if strings.Contains(route, "/") {
			route += "/32"
		}
		_ = exec.Command("sudo", "route", "add", "-net", route, t.ipv4()).Run()
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
