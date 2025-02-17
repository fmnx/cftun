package client

import (
	"fmt"
	"github.com/fmnx/cftun/log"
	tunToArgo "github.com/xjasonlyu/tun2socks/v2/engine"
	"os/exec"
	"runtime"
	"strings"
)

type Tun struct {
	Enable    bool   `yaml:"enable" json:"enable"`
	Name      string `yaml:"name" json:"name"`
	Interface string `yaml:"interface" json:"interface"`
	LogLevel  string `yaml:"log-level" json:"log-level"`
}

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

func (t *Tun) ConfigureTunDevice() {
	if err := exec.Command("ip", "tuntap", "add", "mode", "tun", "dev", t.Name).Run(); err != nil {
		log.Errorln("failed to add tun %s: %w", t.Name, err)
	}

	if err := exec.Command("ip", "addr", "add", "198.18.0.1/15", "dev", t.Name).Run(); err != nil {
		log.Errorln("failed to add IPv4 address to %s: %w", t.Name, err)
	}

	if err := exec.Command("ip", "link", "set", t.Name, "up").Run(); err != nil {
		log.Errorln("failed to set %s up: %w", t.Name, err)
	}
}

func DeleteTunDevice(tunName string) {
	tunToArgo.Stop()
	_ = exec.Command("ip", "link", "set", tunName, "down").Run()
	_ = exec.Command("ip", "tuntap", "del", tunName, "mode", "tun").Run()
}

func (c *Config) Run() {
	if c.Tun != nil && c.Tun.Enable && runtime.GOOS == "linux" {
		c.Tun.ConfigureTunDevice()
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
