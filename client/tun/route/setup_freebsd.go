//go:build freebsd

package route

import (
	"fmt"
	"github.com/fmnx/cftun/log"
	"os/exec"
	"strings"
)

func getDefaultGateway(is6 bool) (string, error) {
	var cmd *exec.Cmd
	if is6 {
		cmd = exec.Command("netstat", "-rn", "-f", "inet6")
	} else {
		cmd = exec.Command("netstat", "-rn", "-f", "inet")
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "default") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1], nil
			}
		}
	}
	return "", fmt.Errorf("no IPv6 default gateway found")
}

func getIPv4DefaultGateway() (gateway string, err error) {
	return getDefaultGateway(false)
}

func getIPv6DefaultGateway() (gateway string, err error) {
	return getDefaultGateway(true)
}

func configureAddressImpl(tunName, ipv4, ipv6 string) {
	log.Infoln("configureAddressImpl not implemented on FreeBSD")
}

func configureRouteImpl(tunName, ipv4, ipv6 string, routes, exRoutes []string) {
	log.Infoln("configureRouteImpl not implemented on FreeBSD")
}
