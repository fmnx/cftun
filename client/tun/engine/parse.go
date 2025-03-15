package engine

import (
	"fmt"
	"net/netip"
	"net/url"
	"runtime"
	"strings"

	"github.com/fmnx/cftun/client/tun/core/device"
	"github.com/fmnx/cftun/client/tun/core/device/fdbased"
	"github.com/fmnx/cftun/client/tun/core/device/tun"
)

func parseDevice(s string, mtu uint32) (device.Device, error) {
	if !strings.Contains(s, "://") {
		s = fmt.Sprintf("%s://%s", tun.Driver /* default driver */, s)
	}

	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	driver := strings.ToLower(u.Scheme)

	switch driver {
	case fdbased.Driver:
		return parseFD(u, mtu)
	case tun.Driver:
		return parseTUN(u, mtu)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
}

func parseFD(u *url.URL, mtu uint32) (device.Device, error) {
	offset := 0
	// fd offset in ios
	// https://stackoverflow.com/questions/69260852/ios-network-extension-packet-parsing/69487795#69487795
	if runtime.GOOS == "ios" {
		offset = 4
	}
	return fdbased.Open(u.Host, mtu, offset)
}

func parseMulticastGroups(s string) (multicastGroups []netip.Addr, _ error) {
	for _, ip := range strings.Split(s, ",") {
		if ip = strings.TrimSpace(ip); ip == "" {
			continue
		}
		addr, err := netip.ParseAddr(ip)
		if err != nil {
			return nil, err
		}
		if !addr.IsMulticast() {
			return nil, fmt.Errorf("invalid multicast IP: %s", addr)
		}
		multicastGroups = append(multicastGroups, addr)
	}
	return
}
