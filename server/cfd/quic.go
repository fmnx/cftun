package cfd

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/quic-go/quic-go"
	"net"
	"net/netip"
	"runtime"
	"sync"
	"time"
)

const (
	HandshakeIdleTimeout = 5 * time.Second
	MaxIdleTimeout       = 5 * time.Second
	MaxIdlePingPeriod    = 1 * time.Second

	// MaxIncomingStreams is 2^60, which is the maximum supported value by Quic-Go
	MaxIncomingStreams = 1 << 60
)

var (
	portForConnIndex = make(map[uint8]int, 0)
	portMapMutex     sync.Mutex
)

type wrapConn struct {
	quic.Connection
	udpConn *net.UDPConn
}

func (w *wrapConn) CloseWithError(errorCode quic.ApplicationErrorCode, reason string) error {
	err := w.Connection.CloseWithError(errorCode, reason)
	w.udpConn.Close()
	return err
}

func DialQuic(
	ctx context.Context,
	quicConfig *quic.Config,
	tlsConfig *tls.Config,
	edgeAddr netip.AddrPort,
	localAddr net.IP,
	connIndex uint8,
) (quic.Connection, error) {
	udpConn, err := createUDPConnForConnIndex(connIndex, localAddr, edgeAddr)
	if err != nil {
		return nil, err
	}

	conn, err := quic.Dial(ctx, udpConn, net.UDPAddrFromAddrPort(edgeAddr), tlsConfig, quicConfig)
	if err != nil {
		udpConn.Close()
		return nil, err
	}

	conn = &wrapConn{
		conn,
		udpConn,
	}
	return conn, nil
}

func createUDPConnForConnIndex(connIndex uint8, localIP net.IP, edgeIP netip.AddrPort) (*net.UDPConn, error) {
	portMapMutex.Lock()
	defer portMapMutex.Unlock()

	listenNetwork := "udp"
	if runtime.GOOS == "darwin" {
		if edgeIP.Addr().Is4() {
			listenNetwork = "udp4"
		} else {
			listenNetwork = "udp6"
		}
	}

	if port, ok := portForConnIndex[connIndex]; ok {
		if udpConn, err := net.ListenUDP(listenNetwork, &net.UDPAddr{IP: localIP, Port: port}); err == nil {
			return udpConn, nil
		}
	}

	udpConn, err := net.ListenUDP(listenNetwork, &net.UDPAddr{IP: localIP, Port: 0})
	if err == nil {
		udpAddr, ok := (udpConn.LocalAddr()).(*net.UDPAddr)
		if !ok {
			return nil, fmt.Errorf("unable to cast to udpConn")
		}
		portForConnIndex[connIndex] = udpAddr.Port
	} else {
		delete(portForConnIndex, connIndex)
	}

	return udpConn, err
}
