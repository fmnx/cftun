//go:build windows

package route

import (
	"fmt"
	"github.com/fmnx/cftun/log"
	"os/exec"
	"strconv"
	"strings"
)

func getDefaultGateway(is6 bool) (idx, gateway string) {
	var substr string
	var cmd *exec.Cmd
	if is6 {
		substr = "::/0"
		cmd = exec.Command("netsh", "interface", "ipv6", "show", "route")
	} else {
		substr = "0.0.0.0/0"
		cmd = exec.Command("netsh", "interface", "ipv4", "show", "route")
	}
	out, err := cmd.Output()
	if err != nil {
		return
	}
	lines := strings.Split(string(out), "\n")
	metric := int64(65536)
	for _, line := range lines {
		if strings.Contains(line, substr) {
			parts := strings.Fields(line)
			if len(parts) >= 6 {
				newMetric, pe := strconv.ParseInt(parts[2], 10, 64)
				if pe != nil || newMetric >= metric {
					continue
				}
				metric = newMetric
				idx, gateway = parts[4], parts[5] // 网关地址
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
	// netsh interface ipv4 set address name="wintun" source=static addr=192.168.123.1 mask=255.255.255.0
	if err := exec.Command("netsh", "interface", "ipv4", "set", "address", tunName, "static", ipv4).Run(); err != nil {
		log.Errorln("failed to add ipv4 address for tun device %s: %w", tunName, err)
	}

	// netsh interface ipv6 add address "tun0" fd12:3456:789a::1/64
	if err := exec.Command("netsh", "interface", "ipv6", "add", "address", tunName, ipv6).Run(); err != nil {
		log.Errorln("failed to add ipv6 address for tun device %s: %w", tunName, err)
	}

	// netsh interface ipv4 set dnsservers name="wintun" static address=8.8.8.8 register=none validate=no
	if err := exec.Command("netsh", "interface", "ipv4", "set", "dnsservers", fmt.Sprintf("name=%s", tunName),
		"static", "address=8.8.8.8", "register=none", "validate=no").Run(); err != nil {
		log.Errorln("failed to set dns server for tun %s: %w", tunName, err)
	}
}

func configureRouteImpl(tunName, ipv4, ipv6 string, routes, exRoutes []string) {
	for _, route := range routes {
		if strings.Contains(route, ":") {
			if !strings.Contains(route, "/") {
				route += "/128"
			}
			_ = exec.Command("netsh", "interface", "ipv6", "add", "route", route, tunName, ipv6, "metric=1").Run()
			continue
		}
		if !strings.Contains(route, "/") {
			route += "/32"
		}
		_ = exec.Command("netsh", "interface", "ipv4", "add", "route", route, tunName, ipv4, "metric=1").Run()
	}

	if len(exRoutes) > 0 {
		idx4, gateway4 := getIPv4DefaultGateway()
		idx6, gateway6 := getIPv6DefaultGateway()

		for _, route := range exRoutes {
			if strings.Contains(route, ":") {
				if !strings.Contains(route, "/") {
					route += "/128"
				}
				_ = exec.Command("netsh", "interface", "ipv6", "add", "route", route, idx6, gateway6, "metric=1").Run()
				continue
			}
			if !strings.Contains(route, "/") {
				route += "/32"
			}
			_ = exec.Command("netsh", "interface", "ipv4", "add", "route", route, idx4, gateway4, "metric=1").Run()
		}
	}
}
