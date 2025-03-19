package cfd

import (
	"context"
	"errors"
	"fmt"
	"github.com/fmnx/cftun/log"
	"github.com/quic-go/quic-go"
	"net"
	"time"
)

type DialFunc func(network string, address string) (net.Conn, error)

type Proxy struct {
	DialFunc DialFunc
	Proxy4   bool
	Proxy6   bool
}

func (d *Proxy) Dial(network, address string) (net.Conn, error) {
	if isIPv6 := address[0] == '['; (isIPv6 && d.Proxy6) || (!isIPv6 && d.Proxy4) {
		return d.DialFunc(network, address)
	}
	return net.Dial(network, address)
}

type QuicConnection struct {
	conn      quic.Connection
	connIndex uint8

	rpcTimeout  time.Duration
	gracePeriod time.Duration

	proxy *Proxy
}

func NewTunnelConnection(
	conn quic.Connection,
	connIndex uint8,
	rpcTimeout time.Duration,
	gracePeriod time.Duration,
	proxy *Proxy,
) (*QuicConnection, error) {
	return &QuicConnection{
		conn:        conn,
		connIndex:   connIndex,
		rpcTimeout:  rpcTimeout,
		gracePeriod: gracePeriod,
		proxy:       proxy,
	}, nil
}

func (q *QuicConnection) Serve(ctx context.Context, credentials *Credentials, connOptions *ConnectionOptions) error {
	c, err := q.conn.OpenStream()
	if err != nil {
		return fmt.Errorf("failed to open a registration control stream: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		connectionDetail, err := RegisterConnection(
			ctx,
			c,
			q.connIndex,
			credentials,
			connOptions)

		if err != nil {
			log.Errorln(err.Error())
		}
		if connectionDetail != nil && connectionDetail.TunnelIsRemotelyManaged {
			println(connectionDetail.Location)
			break
		}
		time.Sleep(1 * time.Second)
	}

	err = q.acceptStream(ctx)

	return err

}

func (q *QuicConnection) acceptStream(ctx context.Context) error {
	defer q.Close()
	for {
		quicStream, err := q.conn.AcceptStream(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("failed to accept QUIC stream: %w", err)
		}
		go q.handleQuicStream(quicStream)
	}
}

func (q *QuicConnection) handleQuicStream(quicStream quic.Stream) {
	ctx := quicStream.Context()
	stream := &SafeStreamCloser{
		stream: quicStream,
	}
	defer stream.Close()

	noCloseStream := &nopCloserReadWriter{ReadWriteCloser: stream}

	n, err := noCloseStream.Read(make([]byte, 6)) //ignore 6 byte
	if err != nil || n != 6 {
		println(err.Error())
		return
	}

	requestServerStream := &RequestServerStream{ReadWriteCloser: noCloseStream}

	var remoteConn net.Conn
	network, address, err := requestServerStream.Accept()
	if err != nil {
		return
	}
	if network != "" && address != "" {
		remoteConn, err = q.DialWithRetry(network, address, 3)
		if err != nil {
			return
		}
	}

	wsCtx, cancel := context.WithCancel(ctx)
	wsConn := NewConn(wsCtx, requestServerStream)
	defer wsConn.Close()
	defer cancel()

	q.handleConn(ctx, cancel, wsConn, remoteConn)

}

func (q *QuicConnection) handleConn(ctx context.Context, cancel context.CancelFunc, wsConn *Conn, remoteConn net.Conn) {
	buf := make([]byte, 32<<10)

	if remoteConn == nil {
		nr, err := wsConn.Read(buf)
		if err != nil {
			return
		}
		packet, err := Decode(buf[:nr])
		if err != nil {
			return
		}
		network := packet.protocol()
		address := packet.address()
		remoteConn, err = q.DialWithRetry(network, address, 3)
		if err != nil {
			return
		}

		nw, err := remoteConn.Write(packet.Payload)
		if err != nil {
			return
		}
		if nw != len(packet.Payload) {
			return
		}
	}

	go handleRemoteConn(ctx, cancel, remoteConn, wsConn)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			nr, err := wsConn.Read(buf)
			if err != nil {
				return
			}

			//if nr == 2 && buf[0] == 137 && buf[1] == 0 {
			//	println("recv client ping msg")
			//	_, _ = wsConn.Write([]byte{'p', 'o', 'n', 'g'})
			//	continue
			//}

			nw, err := remoteConn.Write(buf[:nr])
			if err != nil {
				return
			}
			if nw != nr {
				return
			}
		}

	}

}

func handleRemoteConn(ctx context.Context, cancel context.CancelFunc, remoteConn net.Conn, wsConn *Conn) {
	var err error

	defer func() {
		cancel()
		wsConn.Close()
		_ = remoteConn.Close()
		//if err != nil {
		//	fmt.Printf("handleRemoteConn exit: %v\n", err)
		//
		//}
	}()

	setReadDeadline := func(c net.Conn) error { return nil }
	if _, ok := remoteConn.(*net.UDPConn); ok {
		udpTimeout := 60 * time.Second
		if remoteConn.RemoteAddr().(*net.UDPAddr).Port == 53 { // DNS query
			udpTimeout = 1 * time.Second
		}
		setReadDeadline = func(c net.Conn) error {
			return c.SetReadDeadline(time.Now().Add(udpTimeout))
		}
	}

	buf := make([]byte, 32<<10)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			err = setReadDeadline(remoteConn)
			if err != nil {
				return
			}
			var nr, nw int
			nr, err = remoteConn.Read(buf)
			if err != nil {
				return
			}
			nw, err = wsConn.Write(buf[:nr])
			if err != nil {
				return
			}
			if nw != nr {
				err = errors.New("short write")
				return
			}
		}
	}
}

func (q *QuicConnection) Close() {
	_ = q.conn.CloseWithError(0, "")
}

func (q *QuicConnection) DialWithRetry(network, address string, maxRetries int) (net.Conn, error) {
	var (
		conn net.Conn
		err  error
	)

	for i := 0; i < maxRetries; i++ {
		conn, err = q.proxy.Dial(network, address)
		if err == nil {
			return conn, nil
		}

		if !isRetryableError(err) {
			return nil, fmt.Errorf("non-retryable error: %w", err)
		}

		fmt.Printf("Attempt %d failed: %v. Retrying...\n", i+1, err)

		time.Sleep(100 * time.Millisecond)
	}

	return nil, fmt.Errorf("after %d retries: %w", maxRetries, err)
}

func isRetryableError(err error) bool {
	if neterr, ok := err.(net.Error); ok {
		return neterr.Timeout()
	}
	return false
}
