package tunnel

import (
	"github.com/fmnx/cftun/client/tun/core/adapter"
	"github.com/fmnx/cftun/client/tun/log"
	M "github.com/fmnx/cftun/client/tun/metadata"
)

// TODO: Port Restricted NAT support.
func (t *Tunnel) handleUDPConn(originConn adapter.UDPConn) {
	defer originConn.Close()

	id := originConn.ID()
	srcIP := parseTCPIPAddress(id.RemoteAddress)
	dstIP := parseTCPIPAddress(id.LocalAddress)

	ipVersion := uint8(6)
	if dstIP.Is4() {
		ipVersion = 4
	}
	metadata := &M.Metadata{
		Network:   M.UDP,
		IPVersion: ipVersion,
		SrcIP:     srcIP,
		SrcPort:   id.RemotePort,
		DstIP:     dstIP,
		DstPort:   id.LocalPort,
	}

	remoteConn, err := t.Dialer().Dial(metadata)
	if err != nil {
		log.Warnf("[UDP] dial %s: %v", metadata.DestinationAddress(), err)
		return
	}
	log.Infof("[UDP] %s <-> %s", metadata.SourceAddress(), metadata.DestinationAddress())

	pipe(originConn, remoteConn)
}
