package client

import (
	"fmt"
	"github.com/fmnx/cftun/client/tun/transport/argo"
	"strings"
)

type Tunnel struct {
	Listen   string `yaml:"listen" json:"listen"`
	Remote   string `yaml:"remote" json:"remote"`
	Url      string `yaml:"url" json:"url"`
	Protocol string `yaml:"protocol" json:"protocol"`
	Timeout  int    `yaml:"timeout" json:"timeout"`
}

type Config struct {
	CdnIp     string    `yaml:"cdn-ip" json:"cdn-ip"`
	CdnPort   int       `yaml:"cdn-port" json:"cdn-port"`
	PoolSize  int32     `yaml:"pool-size" json:"pool-size"`
	GlobalUrl string    `yaml:"global-url" json:"global-url"`
	Scheme    string    `yaml:"scheme" json:"scheme"`
	Tunnels   []*Tunnel `yaml:"tunnels" json:"tunnels"`
	Tun       *Tun      `yaml:"tun" json:"tun"`
}

func (c *Config) Run() {
	if c.Tun != nil && c.Tun.Enable {
		params := &argo.Params{
			Scheme:   c.getScheme(),
			CdnIP:    c.CdnIp,
			Url:      c.GlobalUrl,
			Port:     c.getPort(),
			PoolSize: c.getPoolSize(),
		}
		c.Tun.Run(params)
	}

	for _, tunnel := range c.Tunnels {
		if tunnel.Url == "" {
			tunnel.Url = c.GlobalUrl
		}
		switch tunnel.Protocol {
		case "udp":
			go UdpListen(c, tunnel)
		case "tcp":
			go TcpListen(c, tunnel)
		default:
			tunnel.Protocol = "tcp"
			go TcpListen(c, tunnel)
		}
	}
}

func (c *Config) getAddress() string {
	if strings.Contains(c.CdnIp, ":") && !strings.Contains(c.CdnIp, "[") {
		return fmt.Sprintf("[%s]:%d", c.CdnIp, c.getPort())
	}
	return fmt.Sprintf("%s:%d", c.CdnIp, c.getPort())
}

func (c *Config) getPort() int {
	if c.CdnPort == 0 {
		return 443
	}
	return c.CdnPort
}

func (c *Config) getPoolSize() int32 {
	if c.PoolSize == 0 {
		return 10
	}
	return c.PoolSize
}

func (c *Config) getScheme() string {
	if c.Scheme != "" {
		return c.Scheme
	}
	switch c.getPort() {
	case 80, 8080, 8880, 2052, 2082, 2086, 2095:
		return "ws"
	default:
		return "wss"
	}
}
