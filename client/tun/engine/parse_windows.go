package engine

import (
	"net/url"

	"github.com/gorilla/schema"
	"golang.org/x/sys/windows"
	wun "golang.zx2c4.com/wireguard/tun"

	"github.com/fmnx/cftun/client/tun/core/device"
	"github.com/fmnx/cftun/client/tun/core/device/tun"
)

func init() {
	wun.WintunTunnelType = "argotunnel"
}

func parseTUN(u *url.URL, mtu uint32) (device.Device, error) {
	opts := struct {
		GUID string
	}{}
	if err := schema.NewDecoder().Decode(&opts, u.Query()); err != nil {
		return nil, err
	}
	if opts.GUID != "" {
		guid, err := windows.GUIDFromString(opts.GUID)
		if err != nil {
			return nil, err
		}
		wun.WintunStaticRequestedGUID = &guid
	}
	return tun.Open(u.Host, mtu)
}
