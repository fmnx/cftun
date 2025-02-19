package server

import (
	"github.com/cloudflare/cloudflared/ingress"
	"github.com/fmnx/cftun/log"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
	"net/netip"
	"strings"
)

type Warp struct {
	Endpoint   string `yaml:"endpoint" json:"endpoint"`
	IPv4       string `yaml:"ipv4" json:"ipv4"`
	IPv6       string `yaml:"ipv6" json:"ipv6"`
	PrivateKey string `yaml:"private-key" json:"private-key"`
	PublicKey  string `yaml:"public-key" json:"public-key"`
	Proxy4     bool   `yaml:"proxy4" json:"proxy4"`
	Proxy6     bool   `yaml:"proxy6" json:"proxy6"`
}

func (w *Warp) verify() bool {
	return w.Endpoint != "" && w.IPv4 != "" && w.PrivateKey != "" && w.PublicKey != ""
}

func (w *Warp) Run() {

	if !w.verify() {
		log.Fatalln("The warp parameter is incorrect.")
	}

	if strings.Contains(w.IPv4, "/") {
		w.IPv4 = strings.Split(w.IPv4, "/")[0]
	}
	if strings.Contains(w.IPv6, "/") {
		w.IPv6 = strings.Split(w.IPv6, "/")[0]
	}

	localAddress := []netip.Addr{netip.MustParseAddr(w.IPv4)}
	if w.IPv6 != "" {
		localAddress = append(localAddress, netip.MustParseAddr(w.IPv6))
	}
	tunDev, tnet, err := netstack.CreateNetTUN(
		localAddress,
		[]netip.Addr{},
		1280,
	)
	if err != nil {
		log.Fatalln(err.Error())
	}

	bind := conn.NewStdNetBind()
	logger := device.NewLogger(1, "")
	dev := device.NewDevice(tunDev, bind, logger)
	dev.SetPrivateKey(w.PrivateKey)
	peer := dev.SetPublicKey(w.PublicKey)
	dev.SetEndpoint(peer, w.Endpoint).SetAllowedIP(peer)
	peer.HandlePostConfig()

	ingress.Warp.Set(tnet.DialContext, w.Proxy4, w.Proxy6)
}
