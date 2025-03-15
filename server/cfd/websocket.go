package cfd

import (
	"context"
	"errors"
	gobwas "github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"io"
	"sync"
	"time"
)

type PingPeriodContext string

const (
	defaultPongWait      = 60 * time.Second
	defaultPingPeriod    = (defaultPongWait * 9) / 10
	PingPeriodContextKey = PingPeriodContext("pingPeriod")
)

type Conn struct {
	rw        io.ReadWriter
	writeLock sync.Mutex
	done      bool
}

func NewConn(ctx context.Context, rw io.ReadWriter) *Conn {
	c := &Conn{
		rw: rw,
	}
	go c.pinger(ctx)
	return c
}

func (c *Conn) pingPeriod(ctx context.Context) time.Duration {
	if val := ctx.Value(PingPeriodContextKey); val != nil {
		if period, ok := val.(time.Duration); ok {
			return period
		}
	}
	return defaultPingPeriod
}

func (c *Conn) ping() (bool, error) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	if c.done {
		return true, nil
	}

	return false, wsutil.WriteServerMessage(c.rw, gobwas.OpPing, []byte{})
}

func (c *Conn) pinger(ctx context.Context) {
	pongMessge := wsutil.Message{
		OpCode:  gobwas.OpPong,
		Payload: []byte{},
	}

	ticker := time.NewTicker(c.pingPeriod(ctx))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			done, err := c.ping()
			if done {
				return
			}
			if err != nil {
				println("failed to write ping message")
			}
			if err := wsutil.HandleClientControlMessage(c.rw, pongMessge); err != nil {
				println("failed to write pong message")
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *Conn) Close() {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	c.done = true
}

// Read will read messages from the websocket connection
func (c *Conn) Read(reader []byte) (int, error) {
	data, err := wsutil.ReadClientBinary(c.rw)
	if err != nil {
		return 0, err
	}
	return copy(reader, data), nil
}

// Write will write messages to the websocket connection.
// It will not write to the connection after Close is called to fix TUN-5184
func (c *Conn) Write(p []byte) (int, error) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	if c.done {
		return 0, errors.New("write to closed websocket connection")
	}

	if err := wsutil.WriteServerBinary(c.rw, p); err != nil {
		return 0, err
	}

	return len(p), nil
}
