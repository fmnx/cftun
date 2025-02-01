package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/fmnx/cftun/log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

type Tunnel struct {
	Listen   string `yaml:"listen" json:"listen"`
	Host     string `yaml:"host" json:"host"`
	Path     string `yaml:"path" json:"path"`
	Protocol string `yaml:"protocol" json:"protocol"`
	Timeout  int    `yaml:"timeout" json:"timeout"`
}

type RawConfig struct {
	CdnIp   string   `yaml:"cdn_ip" json:"cdn_ip"`
	Host    string   `yaml:"host" json:"host"`
	Tunnels []Tunnel `yaml:"tunnels" json:"tunnels"`
}

func parseConfig(configFile string) (*RawConfig, error) {
	if configFile == "" {
		currentDir, _ := os.Getwd()
		configFile = filepath.Join(currentDir, "config.json")
	}
	buf, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	if len(buf) == 0 {
		return nil, fmt.Errorf("configuration file %s is empty", configFile)
	}
	rawCfg := &RawConfig{}
	if err := json.Unmarshal(buf, rawCfg); err != nil {
		return nil, err
	}
	return rawCfg, nil

}

var (
	configFile string
	Debug      bool
)

func init() {
	flag.StringVar(&configFile, "f", "./config.json", "config file.")
	flag.BoolVar(&Debug, "debug", false, "print logs.")
	flag.Parse()
}

func main() {

	rawConfig, err := parseConfig(configFile)
	if err != nil {
		log.Fatalln("Failed to parse config file: ", err.Error())
	}

	// ./wstun

	for _, tunnel := range rawConfig.Tunnels {
		host := rawConfig.Host
		if tunnel.Host != "" {
			host = tunnel.Host
			println(host)
		}
		switch tunnel.Protocol {
		case "udp":
			go udpListen(tunnel.Listen, rawConfig.CdnIp, host, tunnel.Path, tunnel.Timeout)
		case "tcp":
			go tcpListen(tunnel.Listen, rawConfig.CdnIp, host, tunnel.Path)
		default:
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
