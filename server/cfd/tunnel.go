package cfd

import (
	"context"
	"fmt"
	"github.com/fmnx/cftun/log"
	"github.com/quic-go/quic-go"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"
)

type EdgeTunnelServer struct {
	Token        string
	HaConn       int
	EdgeIPS      chan netip.AddrPort
	EdgeBindAddr net.IP
	NsResult     []string
	Proxy        *Proxy
	ClientInfo   *ClientInfo
	mu           sync.Mutex
}

func (e *EdgeTunnelServer) getEdgeIP(index int) netip.AddrPort {
	e.mu.Lock()
	defer e.mu.Unlock()
	select {
	case addrPort := <-e.EdgeIPS:
		return addrPort
	default:
		if len(e.NsResult) == 0 {
			ips, err := net.LookupHost("region1.v2.argotunnel.com")
			if err == nil {
				e.NsResult = ips
			}

			ips2, err := net.LookupHost("region2.v2.argotunnel.com")
			if err == nil {
				e.NsResult = append(e.NsResult, ips2...)
			}
		}

		var addr string
		if len(e.NsResult) > index {
			ip := e.NsResult[index]
			if strings.Contains(ip, ":") {
				addr = fmt.Sprintf("[%s]:7844", ip)
			} else {
				addr = fmt.Sprintf("%s:7844", ip)
			}
		} else {
			addr = fmt.Sprintf("198.41.192.%d:7844", index)
		}

		addrPort, _ := netip.ParseAddrPort(addr)
		return addrPort
	}
}

func (e *EdgeTunnelServer) Serve(connIndex int) error {

	ctx := context.Background()

	rpcTimeout := 5 * time.Second
	gracePeriod := 30 * time.Second
	edgeAddr := e.getEdgeIP(connIndex)
	tunnelToken, err := ParseToken(e.Token)
	if err != nil {
		return err
	}

	connOptions := &ConnectionOptions{
		Client:          e.ClientInfo,
		ReplaceExisting: true,
	}

	return e.serveQUIC(ctx,
		edgeAddr,
		connOptions,
		tunnelToken.Credentials(),
		rpcTimeout,
		gracePeriod,
		uint8(connIndex))
}

func (e *EdgeTunnelServer) serveQUIC(
	ctx context.Context,
	edgeAddr netip.AddrPort,
	connOptions *ConnectionOptions,
	credentials *Credentials,
	rpcTimeout,
	gracePeriod time.Duration,
	connIndex uint8,
) (err error) {

	tlsConfig, err := CreateTunnelConfig("quic.cftunnel.com")
	if err != nil {
		return fmt.Errorf("unable to create TLS config to connect with edge: %s", err.Error())
	}
	tlsConfig.NextProtos = []string{"argotunnel"}

	var initialPacketSize uint16 = 1252
	if edgeAddr.Addr().Is4() {
		initialPacketSize = 1232
	}

	quicConfig := &quic.Config{
		HandshakeIdleTimeout:  HandshakeIdleTimeout,
		MaxIdleTimeout:        MaxIdleTimeout,
		KeepAlivePeriod:       MaxIdlePingPeriod,
		MaxIncomingStreams:    MaxIncomingStreams,
		MaxIncomingUniStreams: MaxIncomingStreams,
		EnableDatagrams:       true,
		InitialPacketSize:     initialPacketSize,
	}

	conn, err := DialQuic(
		ctx,
		quicConfig,
		tlsConfig,
		edgeAddr,
		e.EdgeBindAddr,
		connIndex,
	)
	if err != nil {
		log.Errorln("Failed to dial a quic connection")
		return err
	}

	tunnelConn, err := NewTunnelConnection(
		conn,
		connIndex,
		rpcTimeout,
		gracePeriod,
		e.Proxy,
	)
	if err != nil {
		log.Errorln("Failed to create new tunnel connection")
		return err
	}

	return tunnelConn.Serve(ctx, credentials, connOptions)
}
