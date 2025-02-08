package main

import (
	"github.com/fmnx/cftun/log"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	UdpBufSize        = 64 * 1024
	InboundQueueSize  = 1024
	OutboundQueueSize = 65536
)

type InboundData struct {
	len int
	buf []byte
}

type OutboundData struct {
	len      int
	buf      []byte
	srcAddr  net.Addr
	listener net.PacketConn
}

type Connector struct {
	listener     net.PacketConn
	srcAddr      net.Addr
	remoteConn   net.Conn
	udpConns     *sync.Map
	inboundQueue chan *InboundData
	lastRecvTime time.Time
	idleTimeout  time.Duration
	closed       bool
}

var (
	udpBufPool = sync.Pool{
		New: func() any {
			return make([]byte, UdpBufSize)
		},
	}
	outboundQueue = make(chan *OutboundData, OutboundQueueSize)
)

func init() {
	cpus := runtime.NumCPU()
	// Use goroutines to process outbound queue concurrently
	for i := 0; i < cpus; i++ {
		go processOutboundQueue()
	}
}

// Listen on addr.
func udpListen(listenAddr, cfIp, host, path string, udpTimeout int) {
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
			Timeout:   5 * time.Second, // 设置连接超时时间
		}
	}

	// 监听指定网卡源地址
	listener, err := net.ListenPacket("udp", listenAddr)
	if err != nil {
		log.Errorln("UDP listen error: %v\n", err)
		return
	}
	defer func(clientPC net.PacketConn) {
		err = clientPC.Close()
		if err != nil {
			println(err.Error())
		}
	}(listener)

	log.Infoln("UDP listen on %s\n", listenAddr)

	ws := NewWebsocket(dialer, cfIp, host, path)

	udpConns := &sync.Map{}

	for {
		buf := udpBufPool.Get().([]byte)
		n, srcAddr, err := listener.ReadFrom(buf)
		if err != nil {
			udpBufPool.Put(buf)
			log.Errorln(err.Error())
			continue
		}

		if conn, ok := udpConns.Load(srcAddr.String()); ok {
			conn.(*Connector).inboundQueue <- &InboundData{
				len: n,
				buf: buf,
			}
			continue
		}

		go func(srcAddr net.Addr) {
			conn := NewConn(ws, listener, srcAddr, udpTimeout, udpConns)
			if conn == nil {
				udpBufPool.Put(buf)
				return
			}
			udpConns.Store(srcAddr.String(), conn)
			conn.inboundQueue <- &InboundData{
				len: n,
				buf: buf,
			}
		}(srcAddr)

	}
}

// Process inbound queue and send data to remote
func (c *Connector) processInboundQueue() {
	for data := range c.inboundQueue {
		n := data.len
		buf := data.buf

		if _, err := c.remoteConn.Write(buf[:n]); err != nil {
			//log.Errorln("Error writing to remote: %v", err)
		}

		udpBufPool.Put(buf)
	}
}

// Process outbound queue and send data
func processOutboundQueue() {
	for data := range outboundQueue {
		n := data.len
		buf := data.buf
		listener := data.listener
		srcAddr := data.srcAddr

		// Write data to client
		if _, err := listener.WriteTo(buf[:n], srcAddr); err != nil {
			log.Errorln("Error writing to client %v: %v", srcAddr, err)
		}
		udpBufPool.Put(buf)
	}
}

func NewConn(ws *Websocket, listener net.PacketConn, srcAddr net.Addr, udpTimeout int, udpConns *sync.Map) *Connector {
	remoteConn, err := ws.createWebsocketStream()
	if err != nil {
		log.Errorln(err.Error())
		return nil
	}
	connector := &Connector{
		listener:     listener,
		srcAddr:      srcAddr,
		remoteConn:   remoteConn,
		lastRecvTime: time.Now(),
		idleTimeout:  time.Duration(udpTimeout),
		closed:       false,
		inboundQueue: make(chan *InboundData, InboundQueueSize),
		udpConns:     udpConns,
	}
	go connector.handleRemote()
	go connector.healthCheck()
	go connector.processInboundQueue()
	return connector
}

func (c *Connector) ReadFromRemote(b []byte) (n int, err error) {
	c.lastRecvTime = time.Now()
	return c.remoteConn.Read(b)
}

func (c *Connector) healthCheck() {
	for {
		if time.Now().After(c.lastRecvTime.Add(c.idleTimeout * time.Second)) {
			//log.Infoln("UDP: %s -> %s closed.", c.srcAddr.String(), c.listener.LocalAddr().String())
			close(c.inboundQueue)
			c.closed = true
			_ = c.remoteConn.Close()
			c.udpConns.Delete(c.srcAddr.String())
			return
		}
		time.Sleep(c.idleTimeout * time.Second)
	}
}

func (c *Connector) handleRemote() {
	for !c.closed {
		buf := udpBufPool.Get().([]byte)
		n, err := c.ReadFromRemote(buf)
		if err != nil {
			c.closed = true
			c.udpConns.Delete(c.srcAddr.String())
			_ = c.remoteConn.Close()
			return
		}

		// Send data to outbound queue
		outboundQueue <- &OutboundData{
			len:      n,
			buf:      buf,
			listener: c.listener,
			srcAddr:  c.srcAddr,
		}
	}
}
