package metadata

import (
	"fmt"
)

const (
	ICMP = 1
	TCP  = 6
	UDP  = 17
)

type Network uint8

func (n Network) String() string {
	switch n {
	case ICMP:
		return "icmp"
	case TCP:
		return "tcp"
	case UDP:
		return "udp"
	default:
		return fmt.Sprintf("network(%d)", n)
	}
}

func (n Network) MarshalText() ([]byte, error) {
	return []byte(n.String()), nil
}
