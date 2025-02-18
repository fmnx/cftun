package proxy

import (
	"context"
	M "github.com/fmnx/cftun/client/tun2argo/metadata"
	"github.com/fmnx/cftun/client/tun2argo/transport/argo"
	"net"
)

type Argo struct {
	ws *argo.Websocket
}

func (a *Argo) Addr() string {
	return a.ws.Url
}

func (a *Argo) Host() string {
	return a.ws.Address
}

func NewArgo(scheme, cdnIP, url string, port int) *Argo {
	return &Argo{
		ws: argo.NewWebsocket(scheme, cdnIP, url, port),
	}
}

func (a *Argo) DialContext(ctx context.Context, metadata *M.Metadata) (net.Conn, error) {
	c, err := a.ws.CreateWebsocketStream(ctx, "tcp", metadata.DestinationAddress())
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (a *Argo) DialUDP(metadata *M.Metadata) (net.PacketConn, error) {
	c, err := a.ws.CreateWebsocketStream(nil, "udp", metadata.DestinationAddress())
	if err != nil {
		return nil, err
	}
	return &argoPacketConn{Conn: c}, nil
}

type argoPacketConn struct {
	net.Conn
	rAddr net.Addr
}

func (w argoPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, err = w.Conn.Read(p)
	if err != nil {
		return 0, nil, err
	}
	return n, w.rAddr, nil
}

func (w argoPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	n, err = w.Conn.Write(p)
	if err != nil {
		return 0, err
	}
	w.rAddr = addr
	return n, nil
}
