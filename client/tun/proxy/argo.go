package proxy

import (
	"encoding/binary"
	M "github.com/fmnx/cftun/client/tun/metadata"
	"github.com/fmnx/cftun/client/tun/transport/argo"
	"net"
	"sync"
)

type Argo struct {
	ws *argo.Websocket
}

func (a *Argo) Close() {
	a.ws.Close()
}

func (a *Argo) Addr() string {
	return a.ws.Url
}

func (a *Argo) Host() string {
	return a.ws.Address
}

func NewArgo(params *argo.Params) *Argo {
	return &Argo{
		ws: argo.NewWebsocket(params),
	}
}

func (a *Argo) Dial(metadata *M.Metadata) (net.Conn, error) {
	c, headerSent, err := a.ws.Dial(metadata)
	if err != nil {
		return nil, err
	}

	if headerSent {
		return c, nil
	}

	conn := &argoConn{
		Conn: c,
	}

	conn.parseHeader(metadata)

	return conn, nil
}

type argoConn struct {
	net.Conn
	header     []byte
	hdrLen     int
	mu         sync.Mutex
	headerSent bool
}

func (w *argoConn) Read(p []byte) (n int, err error) {
	n, err = w.Conn.Read(p)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (w *argoConn) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.headerSent {
		return w.Conn.Write(p)
	}

	w.headerSent = true
	payload := w.addHeader(p)
	n, err = w.Conn.Write(payload)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *argoConn) parseHeader(metadata *M.Metadata) {
	hdrLen := 8
	if metadata.DstIP.Is6() {
		hdrLen = 20
	}
	header := make([]byte, hdrLen)
	pos := 0
	header[pos] = metadata.IPVersion
	pos++
	header[pos] = byte(metadata.Network)
	pos++
	pos += copy(header[pos:], metadata.DstIP.AsSlice())
	binary.BigEndian.PutUint16(header[pos:], metadata.DstPort)
	pos += 2
	w.header = header
	w.hdrLen = hdrLen
}

func (w *argoConn) addHeader(p []byte) []byte {
	buf := make([]byte, len(p)+w.hdrLen)
	copy(buf, w.header)
	copy(buf[w.hdrLen:], p)
	return buf
}
