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
	len        int
	buf        []byte
	clientAddr net.Addr
}

type Connector struct {
	listener      net.PacketConn
	clientAddr    net.Addr
	remote        net.Conn
	lastTime      time.Time
	timeOut       time.Duration
	closed        bool
	inboundQueue  chan *InboundData
	outboundQueue chan *OutboundData
	udpConns      *sync.Map
}

var (
	udpBufPool = sync.Pool{
		New: func() any {
			return make([]byte, UdpBufSize)
		},
	}
)

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

	cpus := runtime.NumCPU()
	outboundQueue := make(chan *OutboundData, OutboundQueueSize)

	// Use goroutines to process outbound queue concurrently
	for i := 0; i < cpus; i++ {
		go processOutboundQueue(listener, outboundQueue)
	}

	log.Infoln("UDP listen on %s\n", listenAddr)

	ws := NewWebsocket(dialer, cfIp, host, path)

	udpConns := &sync.Map{}

	for {
		buf := udpBufPool.Get().([]byte)
		n, clientAddr, err := listener.ReadFrom(buf)
		if err != nil {
			udpBufPool.Put(buf)
			log.Errorln(err.Error())
			continue
		}

		if conn, ok := udpConns.Load(clientAddr.String()); ok {
			conn.(*Connector).inboundQueue <- &InboundData{
				len: n,
				buf: buf,
			}
			continue
		}

		go func() {
			conn := NewConn(ws, listener, clientAddr, udpTimeout, outboundQueue, udpConns)
			if conn == nil {
				udpBufPool.Put(buf)
				return
			}
			udpConns.Store(clientAddr.String(), conn)
			conn.inboundQueue <- &InboundData{
				len: n,
				buf: buf,
			}
		}()

	}
}

// Process inbound queue and send data to remote
func (c *Connector) processInboundQueue() {
	for data := range c.inboundQueue {
		n := data.len
		buf := data.buf

		if _, err := c.remote.Write(buf[:n]); err != nil {
			//log.Errorln("Error writing to remote: %v", err)
		}

		udpBufPool.Put(buf)
	}
}

// Process outbound queue and send data
func processOutboundQueue(listener net.PacketConn, outboundQueue chan *OutboundData) {
	for data := range outboundQueue {
		n := data.len
		buf := data.buf
		clientAddr := data.clientAddr

		// Write data to client
		if _, err := listener.WriteTo(buf[:n], clientAddr); err != nil {
			log.Errorln("Error writing to client %v: %v", clientAddr, err)
		}
		udpBufPool.Put(buf)
	}
}

func NewConn(ws *Websocket, listener net.PacketConn, clientAddr net.Addr, udpTimeout int, outboundQueue chan *OutboundData, udpConns *sync.Map) *Connector {
	remote, err := ws.createWebsocketStream()
	if err != nil {
		log.Errorln(err.Error())
		return nil
	}
	connector := &Connector{
		listener:      listener,
		clientAddr:    clientAddr,
		remote:        remote,
		lastTime:      time.Now(),
		timeOut:       time.Duration(udpTimeout),
		closed:        false,
		inboundQueue:  make(chan *InboundData, InboundQueueSize),
		outboundQueue: outboundQueue,
		udpConns:      udpConns,
	}
	go connector.handleRemote()
	go connector.healthCheck()
	go connector.processInboundQueue()
	return connector
}

func (c *Connector) ReadFromRemote(b []byte) (n int, err error) {
	c.lastTime = time.Now()
	return c.remote.Read(b)
}

func (c *Connector) healthCheck() {
	for {
		if time.Now().After(c.lastTime.Add(c.timeOut * time.Second)) {
			//log.Infoln("UDP: %s -> %s closed.", c.clientAddr.String(), c.listener.LocalAddr().String())
			close(c.inboundQueue)
			c.closed = true
			_ = c.remote.Close()
			c.udpConns.Delete(c.clientAddr.String())
			return
		}
		time.Sleep(c.timeOut * time.Second)
	}
}

func (c *Connector) handleRemote() {
	for !c.closed {
		buf := udpBufPool.Get().([]byte)
		n, err := c.ReadFromRemote(buf)
		if err != nil {
			c.closed = true
			c.udpConns.Delete(c.clientAddr.String())
			_ = c.remote.Close()
			return
		}

		// Send data to outbound queue
		c.outboundQueue <- &OutboundData{
			len:        n,
			buf:        buf,
			clientAddr: c.clientAddr,
		}
	}
}
