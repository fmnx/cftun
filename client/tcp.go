package client

import (
	"github.com/fmnx/cftun/log"
	"net"
	"sync"
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
	closed bool
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
	}
	tcpConnector.handle()
}

func (t *TcpConnector) handle() {
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
	defer t.wsConn.Close()
	defer t.Close()
	defer bufferPool.Put(buf)
	for !t.closed {
		nr, err := t.conn.Read(buf)
		if err != nil {
			//log.Infoln("handleUpstream: %s", err.Error())
			break
		}
		nw, ew := t.safeWrite(buf[:nr])
		if ew != nil || nw != nr {
			break
			//log.Infoln("handleUpstream: Write to remote failed.")
		}
	}
	return
}

func (t *TcpConnector) handleDownstream() {
	buf := bufferPool.Get().([]byte)
	defer t.conn.Close()
	defer t.Close()
	defer bufferPool.Put(buf)
	for !t.closed {
		nr, err := t.wsConn.Read(buf)
		if err != nil {
			break
		}
		nw, ew := t.conn.Write(buf[:nr])
		if ew != nil || nw != nr {
			//log.Errorln("Write to local failed.")
			break
		}
	}
}

func TcpListen(listenAddr, cfIp, url, scheme string, port int) {
	// 监听指定网卡源地址
	tcpListener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Errorln("TCP listen error: %s", err.Error())
		return
	}
	defer tcpListener.Close()
	log.Infoln("TCP listen on %s", listenAddr)

	errChan := make(chan error)

	ws := NewWebsocket(listenAddr, cfIp, url, scheme, port)

	for {
		conn, err := tcpListener.Accept()
		if err != nil {
			select {
			case errChan <- err:
			default:
			}
			log.Errorln(err.Error())
			return
		}
		go handleTcp(ws, conn)
	}
}
