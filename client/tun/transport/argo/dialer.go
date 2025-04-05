package argo

import (
	"errors"
	"fmt"
	"github.com/fmnx/cftun/client/tun/dialer"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"strings"
	"time"
)

type Websocket struct {
	headers  http.Header
	cdnIP    string
	Url      string
	Scheme   string
	Address  string
	wsDialer *websocket.Dialer

	stopChan chan struct{}
	connPool chan net.Conn
}

func NewWebsocket(scheme, cdnIP, Url string, port int) *Websocket {

	hostPath := strings.Split(Url, "/")
	host := hostPath[0]
	path := ""
	if len(hostPath) > 1 {
		path = hostPath[1]
	}

	wsDialer := &websocket.Dialer{
		TLSClientConfig:   nil,
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  time.Second,
		ReadBufferSize:    32 << 10,
		WriteBufferSize:   32 << 10,
		EnableCompression: true,
	}

	address := fmt.Sprintf("%s:%d", cdnIP, port)
	if strings.Contains(cdnIP, ":") {
		address = fmt.Sprintf("[%s]:%d", cdnIP, port)
	}
	wsDialer.NetDial = func(network, addr string) (net.Conn, error) {
		if cdnIP != "" {
			return dialer.Dial(network, address)
		}
		return dialer.Dial(network, addr)
	}

	headers := make(http.Header)
	headers.Set("Host", host)
	headers.Set("User-Agent", "DEV")

	poolSize := 30
	ws := &Websocket{
		wsDialer: wsDialer,
		headers:  headers,
		cdnIP:    cdnIP,
		Scheme:   scheme,
		Address:  address,
		Url:      fmt.Sprintf("%s://%s%s", scheme, host, path),

		stopChan: make(chan struct{}),
		connPool: make(chan net.Conn, poolSize),
	}
	return ws
}

func (w *Websocket) Close() {
	close(w.stopChan)
	for conn := range w.connPool {
		go conn.Close()
	}
}

func (w *Websocket) preDial() {
	select {
	case <-w.stopChan:
		return
	default:
		conn, err := w.connect()
		if err != nil {
			return
		}
		select {
		case w.connPool <- conn:
			return
		default:
			_ = conn.Close()
		}
	}
}

func (w *Websocket) connect() (net.Conn, error) {
	wsConn, resp, err := w.wsDialer.Dial(w.Url, w.headers)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	if err != nil {
		return nil, err
	}

	return &GorillaConn{Conn: wsConn}, nil
}

func (w *Websocket) Dial() (net.Conn, error) {
	defer func() { go w.preDial() }()
	select {
	case <-w.stopChan:
		return nil, errors.New("websocket has been closed")
	case conn := <-w.connPool:
		return conn, nil
	default:
		return w.connect()
	}
}
