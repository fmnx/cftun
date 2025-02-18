package argo

import (
	"context"
	"fmt"
	"github.com/fmnx/cftun/client/tun2argo/dialer"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"strings"
)

type Websocket struct {
	headers  http.Header
	cdnIP    string
	Url      string
	Scheme   string
	Address  string
	wsDialer *websocket.Dialer
}

func NewWebsocket(scheme, cdnIP, Url string, port int) *Websocket {

	hostPath := strings.Split(Url, "/")
	host := hostPath[0]
	path := ""
	if len(hostPath) > 1 {
		path = hostPath[1]
	}

	wsDialer := &websocket.Dialer{
		TLSClientConfig: nil,
		Proxy:           http.ProxyFromEnvironment,
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

	return &Websocket{
		wsDialer: wsDialer,
		headers:  headers,
		cdnIP:    cdnIP,
		Scheme:   scheme,
		Address:  address,
		Url:      fmt.Sprintf("%s://%s%s", scheme, host, path),
	}

}

func (w *Websocket) getDialer(ctx context.Context) *websocket.Dialer {
	wsDialer := &websocket.Dialer{}
	wsDialer.NetDial = func(network, addr string) (net.Conn, error) {
		if w.cdnIP != "" {
			return dialer.DialContext(ctx, network, w.Address)
		}
		return dialer.DialContext(ctx, network, addr)
	}
	return wsDialer
}

func (w *Websocket) getHeaders(network, address string) http.Header {
	dst := make(http.Header)
	for k, v := range w.headers {
		dst[k] = v
	}
	dst.Set("Forward-Dest", address)
	dst.Set("Forward-Proto", network)
	return dst
}

func (w *Websocket) CreateWebsocketStream(ctx context.Context, network, address string) (net.Conn, error) {
	wsDialer := w.wsDialer
	if ctx != nil {
		wsDialer = w.getDialer(ctx)
	}
	wsConn, resp, err := wsDialer.Dial(w.Url, w.getHeaders(network, address))

	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	if err != nil {
		return nil, err
	}

	return &GorillaConn{Conn: wsConn}, nil
}
