//go:build unix

package engine

import (
	"net/url"

	"github.com/fmnx/cftun/client/tun/core/device"
	"github.com/fmnx/cftun/client/tun/core/device/tun"
)

func parseTUN(u *url.URL, mtu uint32) (device.Device, error) {
	return tun.Open(u.Host, mtu)
}
