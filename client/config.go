package client

type Tunnel struct {
	Listen   string `yaml:"listen" json:"listen"`
	Url      string `yaml:"url" json:"url"`
	Protocol string `yaml:"protocol" json:"protocol"`
	Timeout  int    `yaml:"timeout" json:"timeout"`
}

type Config struct {
	CdnIp   string   `yaml:"cdn-ip" json:"cdn-ip"`
	CdnPort int      `yaml:"cdn-port" json:"cdn-port"`
	Scheme  string   `yaml:"scheme" json:"scheme"`
	Tunnels []Tunnel `yaml:"tunnels" json:"tunnels"`
}

func (client *Config) Run() {
	if len(client.Tunnels) == 0 {
		return
	}

	if client.CdnPort == 0 {
		client.CdnPort = 443
	}

	if client.Scheme == "" {
		if client.CdnPort == 80 {
			client.Scheme = "ws"
		} else {
			client.Scheme = "wss"
		}
	}

	for _, tunnel := range client.Tunnels {
		switch tunnel.Protocol {
		case "udp":
			go UdpListen(tunnel.Listen, client.CdnIp, tunnel.Url, client.Scheme, client.CdnPort, tunnel.Timeout)
		case "tcp":
			go TcpListen(tunnel.Listen, client.CdnIp, tunnel.Url, client.Scheme, client.CdnPort)
		default:
			go TcpListen(tunnel.Listen, client.CdnIp, tunnel.Url, client.Scheme, client.CdnPort)
		}
	}
}
