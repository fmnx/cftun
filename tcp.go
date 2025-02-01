package main

import (
	"github.com/fmnx/cftun/log"
	"github.com/gorilla/websocket"
	"net"
	"strings"
	"sync"
	"time"
)

const defaultBufferSize = 16 * 1024

var bufferPool = sync.Pool{
	New: func() any {
		return make([]byte, defaultBufferSize)
	},
}

type TcpConnector struct {
	ws     *Websocket
	wsConn net.Conn
	conn   net.Conn
	cache  []byte // 缓存最后一次发送的数据
	closed bool
	newWS  bool
	mu     sync.Mutex
}

func handleTcp(ws *Websocket, conn net.Conn) {
	wsConn, err := ws.createWebsocketStream()
	if err != nil {
		_ = conn.Close()
		return
	}
	tcpConnector := &TcpConnector{
		ws:     ws,
		wsConn: wsConn,
		conn:   conn,
		closed: false,
		newWS:  false,
	}
	tcpConnector.handle()
}

func (t *TcpConnector) handle() {
	//go Relay(t.ws, t.conn)
	go t.handleDownstream()
	go t.handleUpstream()
}

func (t *TcpConnector) Close() {
	t.closed = true
}

func (t *TcpConnector) safeWrite(b []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.wsConn.Write(b)
}

func (t *TcpConnector) handleUpstream() {
	buf := bufferPool.Get().([]byte)
	defer bufferPool.Put(buf)
	defer t.Close()
	defer t.wsConn.Close()
	for !t.closed {
		nr, err := t.conn.Read(buf)
		if err != nil {
			//log.Infoln("handleUpstream: %s", err.Error())
			break
		}
		nw, ew := t.safeWrite(buf[:nr])
		if ew != nil || nw != nr {
			t.newWS = true
			_ = t.wsConn.Close()
			log.Infoln("handleUpstream: recreate Websocket connection.")
			t.wsConn, _ = t.ws.createWebsocketStream()
			nw, ew = t.safeWrite(buf[:nr])
			if ew != nil || nw != nr {
				break
			}
			//log.Infoln("handleUpstream: Write to remote failed.")
		}
		t.cache = buf[:nr]
	}
	return
}

func (t *TcpConnector) handleDownstream() {
	buf := bufferPool.Get().([]byte)
	defer bufferPool.Put(buf)
	defer t.Close()
	defer t.conn.Close()
	for !t.closed {
		nr, err := t.wsConn.Read(buf)
		if err != nil {
			if t.newWS {
				time.Sleep(10 * time.Millisecond)
				t.newWS = false
				continue
			}
			if e, ok := err.(*websocket.CloseError); ok {
				if e.Code == 1006 && !t.closed {
					log.Infoln("handleDownstream: recreate Websocket connection...")
					t.wsConn, err = t.ws.createWebsocketStream()
					if err != nil {
						break
					}
					n, err := t.safeWrite(t.cache)
					if err != nil || n != len(t.cache) {
						break
					}
					continue
				}
			}
			//log.Infoln("handleDownstream: %s", err.Error())
			break
		}
		nw, ew := t.conn.Write(buf[:nr])
		if ew != nil || nw != nr {
			//log.Errorln("Write to local failed.")
			break
		}
	}
}

func tcpListen(listenAddr, cfIp, host, path string) {
	var dialer *net.Dialer
	// 绑定连接cloudflare服务器的网卡
	if !strings.Contains(listenAddr, "0.0.0.0") && !strings.Contains(listenAddr, "127.0.0.1") {

		localIP, _, _ := net.SplitHostPort(listenAddr)
		localAddr := &net.TCPAddr{
			IP:   net.ParseIP(localIP),
			Port: 0, // 端口设为 0，系统自动分配
		}

		dialer = &net.Dialer{
			LocalAddr: localAddr,
			Timeout:   3 * time.Second, // 设置连接超时时间
		}
	}

	// 监听指定网卡源地址
	tcpListener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Errorln("TCP listen error: %s", err.Error())
		return
	}
	defer tcpListener.Close()
	log.Infoln("TCP listen on %s\n", listenAddr)

	errChan := make(chan error)

	ws := NewWebsocket(dialer, cfIp, host, path)

	for {
		conn, err := tcpListener.Accept()
		if err != nil {
			select {
			case errChan <- err:
			default:
			}
			log.Fatalln(err.Error())
		}
		go handleTcp(ws, conn)
	}
}
