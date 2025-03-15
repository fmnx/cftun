// Package tun provides TUN which implemented device.Device interface.
package tun

import (
	"github.com/fmnx/cftun/client/tun/core/device"
)

const Driver = "tun"

func (t *TUN) Type() string {
	return Driver
}

var _ device.Device = (*TUN)(nil)
