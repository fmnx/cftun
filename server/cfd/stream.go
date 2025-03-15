package cfd

import (
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/quic-go/quic-go"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
	capnp "zombiezen.com/go/capnproto2"
	"zombiezen.com/go/capnproto2/pogs"
	"zombiezen.com/go/capnproto2/schemas"
)

var idleTimeoutError = quic.IdleTimeoutError{}

type RequestServerStream struct {
	io.ReadWriteCloser
}

func (rss *RequestServerStream) Accept() (network, address string, err error) {
	request, err := rss.ReadConnectRequestData()
	if err != nil {
		return
	}
	k := sha1.New()
	k.Write([]byte(request.WebsocketKey()))
	k.Write([]byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	wsAccept := base64.StdEncoding.EncodeToString(k.Sum(nil))

	metadata := []Metadata{
		{"HttpStatus", "101"},
		{"HttpHeader:Connection", "Upgrade"},
		{"HttpHeader:Sec-Websocket-Accept", wsAccept},
		{"HttpHeader:Upgrade", "websocket"},
	}

	err = rss.WriteConnectResponseData(metadata...)

	if err == nil {
		network, address = request.Network(), request.Address()
	}
	return
}

func (rss *RequestServerStream) ReadConnectRequestData() (*ConnectRequest, error) {
	if _, err := readVersion(rss); err != nil {
		return nil, err
	}

	msg, err := capnp.NewDecoder(rss).Decode()
	if err != nil {
		return nil, err
	}

	r := &ConnectRequest{}
	if err := r.FromPogs(msg); err != nil {
		return nil, err
	}
	return r, nil
}

// WriteConnectResponseData writes response to a QUIC stream.
func (rss *RequestServerStream) WriteConnectResponseData(metadata ...Metadata) error {
	connectResponse := &ConnectResponse{
		Metadata: metadata,
	}

	msg, err := connectResponse.ToPogs()
	if err != nil {
		return err
	}

	if err := writeDataStreamPreamble(rss); err != nil {
		return err
	}
	return capnp.NewEncoder(rss).Encode(msg)
}

type ConnectionType uint16

type Metadata struct {
	Key string `capnp:"key"`
	Val string `capnp:"val"`
}

type ConnectRequest struct {
	Dest     string         `capnp:"dest"`
	Type     ConnectionType `capnp:"type"`
	Metadata []Metadata     `capnp:"metadata"`
}

func (r *ConnectRequest) WebsocketKey() string {
	for _, metadata := range r.Metadata {
		if metadata.Key == "HttpHeader:Sec-Websocket-Key" {
			return metadata.Val
		}
	}
	return ""
}

func (r *ConnectRequest) Network() string {
	for _, metadata := range r.Metadata {
		if metadata.Key == "HttpHeader:Forward-Proto" {
			return metadata.Val
		}
	}
	return ""
}

func (r *ConnectRequest) Address() string {
	for _, metadata := range r.Metadata {
		if metadata.Key == "HttpHeader:Forward-Dest" {
			return metadata.Val
		}
	}
	return ""
}

type ConnectRequestProto struct{ capnp.Struct }

func ReadRootConnectRequest(msg *capnp.Message) (ConnectRequestProto, error) {
	root, err := msg.RootPtr()
	return ConnectRequestProto{root.Struct()}, err
}

func (r *ConnectRequest) FromPogs(msg *capnp.Message) error {
	metadata, err := ReadRootConnectRequest(msg)
	if err != nil {
		return err
	}
	return pogs.Extract(r, 0xc47116a1045e4061, metadata.Struct)
}

type ConnectResponse struct {
	Error    string     `capnp:"error"`
	Metadata []Metadata `capnp:"metadata"`
}

type ConnectResponseProto struct{ capnp.Struct }

func NewRootConnectResponse(s *capnp.Segment) (ConnectResponseProto, error) {
	st, err := capnp.NewRootStruct(s, capnp.ObjectSize{DataSize: 0, PointerCount: 2})
	return ConnectResponseProto{st}, err
}

func (r *ConnectResponse) ToPogs() (*capnp.Message, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	root, err := NewRootConnectResponse(seg)
	if err != nil {
		return nil, err
	}

	if err := pogs.Insert(0xb1032ec91cef8727, root.Struct, r); err != nil {
		return nil, err
	}

	return msg, nil
}

type SafeStreamCloser struct {
	lock         sync.Mutex
	stream       quic.Stream
	writeTimeout time.Duration
	closing      atomic.Bool
}

func (s *SafeStreamCloser) Read(p []byte) (n int, err error) {
	return s.stream.Read(p)
}

func (s *SafeStreamCloser) Write(p []byte) (n int, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.writeTimeout > 0 {
		err = s.stream.SetWriteDeadline(time.Now().Add(s.writeTimeout))
		if err != nil {
			println("Error setting write deadline for QUIC stream")
		}
	}
	nBytes, err := s.stream.Write(p)
	if err != nil {
		s.handleWriteError(err)
	}

	return nBytes, err
}

func (s *SafeStreamCloser) handleWriteError(err error) {
	// If we are closing the stream we just ignore any write error.
	if s.closing.Load() {
		return
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			// We don't need to log if what cause the timeout was no network activity.
			if !errors.Is(netErr, &idleTimeoutError) {
				println("Closing quic stream due to timeout while writing")
			}
			// We need to explicitly cancel the write so that it frees all buffers.
			s.stream.CancelWrite(0)
		}
	}
}

func (s *SafeStreamCloser) Close() error {
	// Set this stream to a closing state.
	s.closing.Store(true)

	// Make sure a possible writer does not block the lock forever. We need it, so we can close the writer
	// side of the stream safely.
	_ = s.stream.SetWriteDeadline(time.Now())

	// This lock is eventually acquired despite Write also acquiring it, because we set a deadline to writes.
	s.lock.Lock()
	defer s.lock.Unlock()

	// We have to clean up the receiving stream ourselves since the Close in the bottom does not handle that.
	s.stream.CancelRead(0)
	return s.stream.Close()
}

func (s *SafeStreamCloser) CloseWrite() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.stream.Close()
}

func (s *SafeStreamCloser) SetDeadline(deadline time.Time) error {
	return s.stream.SetDeadline(deadline)
}

type nopCloserReadWriter struct {
	io.ReadWriteCloser

	// for use by Read only
	// we don't need a memory barrier here because there is an implicit assumption that
	// Read calls can't happen concurrently by different go-routines.
	sawEOF bool
	// should be updated and read using atomic primitives.
	// value is read in Read method and written in Close method, which could be done by different
	// go-routines.
	closed uint32
}

func (np *nopCloserReadWriter) Read(p []byte) (n int, err error) {
	if np.sawEOF {
		return 0, io.EOF
	}

	if atomic.LoadUint32(&np.closed) > 0 {
		return 0, fmt.Errorf("closed by handler")
	}

	n, err = np.ReadWriteCloser.Read(p)
	if err == io.EOF {
		np.sawEOF = true
	}

	return
}

func (np *nopCloserReadWriter) Close() error {
	atomic.StoreUint32(&np.closed, 1)

	return nil
}

const SCHEMA = "x\xda\xb4\x911k\x14A\x1c\xc5\xdf\x9b\xcde-\x0e" +
	"\xf7\x86KlT\xc2\x05Q\x13\xdc\x8b\xc9\x09\xa2 \x1c" +
	"\x18A%\xc1\x9b`mX7\x83\x09w\xee\xce\xed\xce" +
	"\x19\xee\x13\xd8\xda\x89\xa5\xbd \x09X\xdb((h!" +
	"\x16\xd6\x0a66\xf9\x04\xb22\x0b\x9b\x83\x90B\x04\xbb" +
	"\xe1\xcd\x9by\xbf\xff\xff5\xbeu\xc5r\xed\x11\x01\xd5" +
	"\xa8M\x17\x17\x9e\x1e\x9c\xf9\xd8\xf6\xf6 C\x16+\x9f" +
	"Z\xf6\xa0\xf5l\x1f5\xe1\x03\xcb/~Q\xbe\xf1\x01" +
	"\xb9\xb7\x0b\x16Q\xf7\xc1\xd4\xcbS\xc3wP!\x8fZ" +
	";-\xfe`\xf3\x06}\xa0y\x8d\xaf\xc1\xe2\xc3\xf8\xeb" +
	"\xf9W\xa7\xdb\xef!C11\x83\x9d\x9f\xceI\xf7\xa8" +
	"\xf9\x9b\xf7\xc0\xe2\xea\xe7/o\x9f\xf7W\xbf\x1fC\xd0" +
	"\x99\x15\xfbl\x86\xa5yA8\x08;J\x12=\xc8\xcc" +
	"t\xbcd\xb2\xd4\xa6K\xc3\xd1N\xbc\xf9X\xdbh+" +
	"\xb2\xd1f\xa9\xc5\xe9\xa0\x1dG&1\xd7o\xa6I\xa2" +
	"c\xbb\xa1s\x13\xa4I\xae{\xa4:\xe1M\x01S\x04" +
	"\xe4\xc2\x0a\xa0\xceyT\x97\x05%9C'\x86w\x01" +
	"u\xc9\xa3\xba-8\xa7\xb3,\xcdX\x87`\x1d,\xaa" +
	"\x14\x00<\x09\xf6<\xb21\xa1\x07\x9d\xf8\xaf\x80\xc3\x91" +
	"\xafs\xeb\xf8\xea\x87|\xb7\x16\x01\xd5\xf5\xa8\xd6\x04+" +
	"\xbc;N[\xf5\xa8z\x82Rp\x86\x02\x90\xeb\x8ey" +
	"\xcd\xa3\xda\x16\x0c\xb6tn+\xe4\xc0\x8e\x8df0)" +
	"\x03d\xf0_'\xd9I\x93\xfb\xfe\xd8\x94\x9b\xae\x97p" +
	"g\x17\xdd\x87rv\x03\xa0\x90r\x1e\x08\xb6\xad5\xc5" +
	"\xae~\x98\xa7q_\x83\xd6\xb7\xb19\x8c\xab\xfdU\xdc" +
	"\xba\xb6s\xe5\xc5\x91J\xe7\x8f\xab\xd4\x89\x17=\xaa+" +
	"\x82~_\x8f\xab\xed\xf8O\xa2Au\xfe\x13\x00\x00\xff" +
	"\xff\x1d\xce\xd1\xb0"

func init() {
	schemas.Register(SCHEMA,
		0xb1032ec91cef8727,
		0xc47116a1045e4061,
		0xc52e1bac26d379c8,
		0xe1446b97bfd1cd37)
}
