package cfd

import (
	"encoding/binary"
	"fmt"
	"net"
)

const (
	ICMP = 1
	TCP  = 6
	UDP  = 17
)

type Packet struct {
	IPVersion uint8
	Protocol  uint8
	DestIP    net.IP
	DestPort  uint16
	Payload   []byte
}

func (p *Packet) protocol() string {
	switch p.Protocol {
	case ICMP:
		return "icmp"
	case TCP:
		return "tcp"
	case UDP:
		return "udp"
	}
	return ""
}

func (p *Packet) address() string {
	if p.IPVersion == 4 {
		return fmt.Sprintf("%s:%d", p.DestIP.String(), p.DestPort)
	}
	return fmt.Sprintf("[%s]:%d", p.DestIP.String(), p.DestPort)
}

func Decode(data []byte) (*Packet, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("data too short")
	}

	p := &Packet{
		IPVersion: data[0],
		Protocol:  data[1],
	}

	offset := 2

	if p.IPVersion == 4 {
		if len(data) < offset+4+2 {
			return nil, fmt.Errorf("invalid IPv4 packet length")
		}
		p.DestIP = data[offset : offset+4]
		offset += 4
	} else if p.IPVersion == 6 {
		if len(data) < offset+16+2 {
			return nil, fmt.Errorf("invalid IPv6 packet length")
		}
		p.DestIP = data[offset : offset+16]
		offset += 16
	} else {
		return nil, fmt.Errorf("unsupported IP version: %d", p.IPVersion)
	}

	if len(data) < offset+4 {
		return nil, fmt.Errorf("missing port fields")
	}
	p.DestPort = binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	p.Payload = data[offset:]

	return p, nil
}
