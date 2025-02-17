package client

import (
	"fmt"
	tunToArgo "github.com/xjasonlyu/tun2socks/v2/engine"
	"runtime"
	"strings"
	"time"
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
	GlobalUrl string    `yaml:"global-url" json:"global-url"`
	Scheme    string    `yaml:"scheme" json:"scheme"`
	Tunnels   []*Tunnel `yaml:"tunnels" json:"tunnels"`
	Tun       *Tun      `yaml:"tun" json:"tun"`
}

func (c *Config) Run() {
	if c.Tun != nil && c.Tun.Enable {
		address := fmt.Sprintf("%s:%d", c.CdnIp, c.CdnPort)
		if strings.Contains(c.CdnIp, ":") && !strings.Contains(c.CdnIp, "[") {
			address = fmt.Sprintf("[%s]:%d", c.CdnIp, c.CdnPort)
		}
		proxy := fmt.Sprintf("argo://%s:%s@%s", c.getScheme(), address, c.GlobalUrl)
		key := &tunToArgo.Key{
			Proxy:     proxy,
			Device:    c.Tun.Name,
			LogLevel:  c.Tun.LogLevel,
			Interface: c.Tun.Interface,
		}
		tunToArgo.Insert(key)
		go tunToArgo.Start()

		switch runtime.GOOS {
		case "linux":
			c.Tun.LinuxConfigure()
		case "windows":
			go func() {
				time.Sleep(1 * time.Second)
				c.Tun.WindowsConfigure()
			}()
		case "darwin":
			go func() {
				time.Sleep(1 * time.Second)
				c.Tun.DarwinConfigure()
			}()
		}
	}

	if len(c.Tunnels) == 0 {
		return
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

func (c *Config) getScheme() string {
	if c.Scheme != "" {
		return c.Scheme
	}
	switch c.getPort() {
	case 80:
		return "ws"
	default:
		return "wss"
	}
}
