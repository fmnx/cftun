package client

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
			go UdpListen(client, tunnel)
		case "tcp":
			go TcpListen(client, tunnel)
		default:
			go TcpListen(client, tunnel)
		}
	}
}
