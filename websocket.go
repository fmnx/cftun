package main

import (
	"fmt"
	cfwebsocket "github.com/cloudflare/cloudflared/websocket"
	"github.com/fmnx/cftun/log"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
)

type Websocket struct {
	wsDialer *websocket.Dialer
	url      string
	headers  http.Header
}

func NewWebsocket(dialer *net.Dialer, cfIp, host, path string) *Websocket {
	wsDialer := &websocket.Dialer{
		TLSClientConfig: nil,
		Proxy:           http.ProxyFromEnvironment,
	}
	dial := net.Dial
	if dialer != nil {
		dial = dialer.Dial
	}

	wsDialer.NetDial = func(network, addr string) (net.Conn, error) {
		// 连接指定的 IP 地址而不是解析域名
		if cfIp != "" {
			return dial(network, fmt.Sprintf("%s:443", cfIp))
		}
		return dial(network, addr)
	}

	headers := make(http.Header)
	headers.Set("Host", host)
	headers.Set("User-Agent", "DEV")

	return &Websocket{
		wsDialer: wsDialer,
		headers:  headers,
		url:      fmt.Sprintf("wss://%s/%s", host, path),
	}

}

func (w *Websocket) createWebsocketStream() (net.Conn, error) {
	wsConn, resp, err := w.wsDialer.Dial(w.url, w.headers)

	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	if err != nil {
		log.Errorln(err.Error())
		return nil, err
	}

	return &cfwebsocket.GorillaConn{Conn: wsConn}, nil
}
