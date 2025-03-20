//go:build linux

package route

import (
	"github.com/fmnx/cftun/log"
	"os/exec"
	"strings"
)

func getDefaultGateway(is6 bool) (gateway, iface string) {
	var cmd *exec.Cmd
	if is6 {
		cmd = exec.Command("ip", "-6", "route", "show", "default")
	} else {
		cmd = exec.Command("ip", "route", "show", "default")
	}
	out, err := cmd.Output()
	if err != nil {
		return
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "default via ") {
			parts := strings.Fields(line)
			if len(parts) >= 5 {
				return parts[2], parts[4]
			}
		}
	}
	return
}

func getIPv4DefaultGateway() (gateway, iface string) {
	return getDefaultGateway(false)
}

func getIPv6DefaultGateway() (gateway, iface string) {
	return getDefaultGateway(true)
}

func configureAddressImpl(tunName, ipv4, ipv6 string) {
	_ = exec.Command("ip", "tuntap", "add", "mode", "tun", "dev", tunName).Run()

	if err := exec.Command("ip", "addr", "add", ipv4, "dev", tunName).Run(); err != nil {
		log.Errorln("failed to add IPv4 address to %s: %w", tunName, err)
	}

	if err := exec.Command("ip", "-6", "addr", "add", ipv6, "dev", tunName).Run(); err != nil {
		log.Errorln("failed to add IPv6 address to %s: %w", tunName, err)
	}

	if err := exec.Command("ip", "link", "set", tunName, "up").Run(); err != nil {
		log.Errorln("failed to set %s up: %w", tunName, err)
	}

}

func configureRouteImpl(tunName, _, _ string, routes, exRoutes []string) {
	for _, route := range routes {
		if strings.Contains(route, ":") {
			_ = exec.Command("ip", "-6", "route", "add", route, "dev", tunName).Run()
			continue
		}
		_ = exec.Command("ip", "route", "add", route, "dev", tunName).Run()
	}

	if len(exRoutes) > 0 {
		gateway4, iface4 := getIPv4DefaultGateway()
		gateway6, iface6 := getIPv6DefaultGateway()
		for _, route := range exRoutes {
			if strings.Contains(route, ":") {
				_ = exec.Command("ip", "-6", "route", "add", route, "via", gateway6, "dev", iface6).Run()
				continue
			}
			_ = exec.Command("ip", "route", "add", route, "via", gateway4, "dev", iface4).Run()
		}
	}
}
