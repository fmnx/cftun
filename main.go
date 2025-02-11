package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/fmnx/cftun/client"
	"github.com/fmnx/cftun/log"
	"github.com/fmnx/cftun/server"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

type RawConfig struct {
	ServerConfig *server.Config `yaml:"server-config" json:"server-config"`
	ClientConfig *client.Config `yaml:"client-config" json:"client-config"`
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

	rawConfig.ClientConfig.Run()
	rawConfig.ServerConfig.Run()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
