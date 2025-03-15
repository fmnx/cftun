package tunnel

import (
	"github.com/fmnx/cftun/client/tun/buffer"
	"github.com/fmnx/cftun/client/tun/core/adapter"
	"github.com/fmnx/cftun/client/tun/log"
	M "github.com/fmnx/cftun/client/tun/metadata"
	"io"
	"net"
	"sync"
)

func (t *Tunnel) handleTCPConn(originConn adapter.TCPConn) {

	id := originConn.ID()
	srcIP := parseTCPIPAddress(id.RemoteAddress)
	dstIP := parseTCPIPAddress(id.LocalAddress)

	ipVersion := uint8(6)
	if dstIP.Is4() {
		ipVersion = 4
	}
	metadata := &M.Metadata{
		Network:   M.TCP,
		IPVersion: ipVersion,
		SrcIP:     srcIP,
		SrcPort:   id.RemotePort,
		DstIP:     dstIP,
		DstPort:   id.LocalPort,
	}

	remoteConn, err := t.Dialer().Dial(metadata)
	if err != nil {
		log.Warnf("[TCP] dial %s: %v", metadata.DestinationAddress(), err)
		return
	}

	defer remoteConn.Close()

	log.Infof("[TCP] %s <-> %s", metadata.SourceAddress(), metadata.DestinationAddress())

	pipe(originConn, remoteConn)

}

func pipe(origin, remote net.Conn) {
	wg := sync.WaitGroup{}
	wg.Add(2)

	go unidirectionalStream(remote, origin, "origin->remote", &wg)
	go unidirectionalStream(origin, remote, "remote->origin", &wg)

	wg.Wait()
}

func unidirectionalStream(dst, src net.Conn, dir string, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := buffer.Get(buffer.RelayBufferSize)
	if _, err := io.CopyBuffer(dst, src, buf); err != nil {
		log.Debugf("[IO] copy data for %s: %v", dir, err)
	}
	buffer.Put(buf)
	_ = src.Close()
	_ = dst.Close()
}
