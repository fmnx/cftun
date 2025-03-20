//go:build darwin

package route

import (
	"github.com/fmnx/cftun/log"
	"os/exec"
	"strings"
)

func getDefaultGateway(is6 bool) (gateway string) {
	var cmd *exec.Cmd
	if is6 {
		cmd = exec.Command("netstat", "-rn", "-f", "inet6")
	} else {

		cmd = exec.Command("netstat", "-rn", "-f", "inet")
	}
	out, err := cmd.Output()
	if err != nil {
		return
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "default") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return
}

func getIPv4DefaultGateway() (gateway string) {
	return getDefaultGateway(false)
}

func getIPv6DefaultGateway() (gateway string) {
	return getDefaultGateway(true)
}

func configureAddressImpl(tunName, ipv4, ipv6 string) {
	// sudo ifconfig utun123 198.18.0.1 198.18.0.1 up
	if err := exec.Command("sudo", "ifconfig", tunName, ipv4, ipv4, "up").Run(); err != nil {
		log.Errorln("failed to add ipv4 address for tun %s: %w", tunName, err)
	}

	// sudo ifconfig utun123 inet6 fd12:3456:789a::1/64 up
	if err := exec.Command("sudo", "ifconfig", tunName, "inet6", ipv6, "up").Run(); err != nil {
		log.Errorln("failed to add ipv6 address for tun %s: %w", tunName, err)
	}
}

func configureRouteImpl(tunName, ipv4, ipv6 string, routes, exRoutes []string) {
	for _, route := range routes {
		// sudo route -inet6 add -net 2001:db8::/32 2001:db8:1::1
		if strings.Contains(route, ":") {
			if !strings.Contains(route, "/") {
				route += "/128"
			}
			_ = exec.Command("sudo", "route", "-inet6", "add", "-net", route, ipv6).Run()
			continue
		}
		// sudo route add -net 1.0.0.0/8 198.18.0.1
		if strings.Contains(route, "/") {
			route += "/32"
		}
		_ = exec.Command("sudo", "route", "add", "-net", route, ipv4).Run()
	}

	if len(exRoutes) > 0 {
		gateway4 := getIPv4DefaultGateway()
		gateway6 := getIPv4DefaultGateway()

		for _, route := range exRoutes {
			if strings.Contains(route, ":") {
				if !strings.Contains(route, "/") {
					route += "/128"
				}
				_ = exec.Command("sudo", "route", "-inet6", "add", "-net", route, gateway6).Run()
				continue
			}
			if !strings.Contains(route, "/") {
				route += "/32"
			}
			_ = exec.Command("sudo", "route", "add", "-net", route, gateway4).Run()
		}
	}
}
