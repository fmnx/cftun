package client

import (
	"fmt"
	cfwebsocket "github.com/cloudflare/cloudflared/websocket"
	"github.com/fmnx/cftun/log"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"strings"
	"time"
)

type Websocket struct {
	wsDialer *websocket.Dialer
	url      string
	headers  http.Header
}

func NewWebsocket(listenAddr, cfIp, url, scheme string, port int) *Websocket {
	host := strings.Split(url, "/")[0]
	wsDialer := &websocket.Dialer{
		TLSClientConfig: nil,
		Proxy:           http.ProxyFromEnvironment,
	}
	dial := net.Dial
	// 绑定监听地址对应的网卡出口
	if !strings.Contains(listenAddr, "0.0.0.0") && !strings.Contains(listenAddr, "127.0.0.1") {
		localIP, _, _ := net.SplitHostPort(listenAddr)
		localAddr := &net.TCPAddr{
			IP:   net.ParseIP(localIP),
			Port: 0,
		}
		dial = (&net.Dialer{
			LocalAddr: localAddr,
			Timeout:   5 * time.Second,
		}).Dial
	}

	wsDialer.NetDial = func(network, addr string) (net.Conn, error) {
		// 连接指定的 IP 地址而不是解析域名
		if cfIp != "" {
			return dial(network, fmt.Sprintf("%s:%d", cfIp, port))
		}
		return dial(network, addr)
	}

	headers := make(http.Header)
	headers.Set("Host", host)
	headers.Set("User-Agent", "DEV")

	return &Websocket{
		wsDialer: wsDialer,
		headers:  headers,
		url:      fmt.Sprintf("%s://%s", scheme, url),
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
