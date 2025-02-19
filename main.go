package main

import (
	"encoding/json"
	"fmt"
	"github.com/cloudflare/cloudflared/cmd/cloudflared/cliutil"
	"github.com/fmnx/cftun/client"
	"github.com/fmnx/cftun/log"
	"github.com/fmnx/cftun/server"
	"github.com/spf13/pflag"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

type RawConfig struct {
	Server *server.Config `yaml:"server" json:"server"`
	Client *client.Config `yaml:"client" json:"client"`
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
	configFile         string
	token              string
	isQuick            bool
	Version            = "unknown"
	BuildDate          = "unknown"
	BuildType          = "DEV"
	CloudflaredVersion = "2025.2.0"
	showVersion        bool
	quickData          = &server.QuickData{}
	tunName            string
)

func init() {
	pflag.StringVarP(&configFile, "config", "c", "./config.json", "")
	pflag.StringVarP(&token, "token", "t", "", "")
	pflag.BoolVarP(&isQuick, "quick", "q", false, "")
	pflag.BoolVarP(&showVersion, "version", "v", false, "")

	pflag.Usage = func() {
		fmt.Println("Usage:")
		fmt.Printf("  -c,--config\tSpecify the path to the config file.(default: \"./config.json\")\n")
		fmt.Printf("  -t,--token\tWhen a token is provided, the configuration file will be ignored and the program will run in server mode only.\n")
		fmt.Printf("  -q,--quick\tTemporary server, no Cloudflare account required, based on try.cloudflare.com.\n")
		fmt.Printf("  -v,--version\tDisplay the current binary file version.\n")
	}
	pflag.Parse()
}

func main() {
	bInfo := cliutil.GetBuildInfo(BuildType, CloudflaredVersion)
	if showVersion {
		printVersion(bInfo)
		return
	}
	if token != "" || isQuick { // command line.
		if isQuick {
			token = "quick"
		}
		srv := &server.Config{
			Token:  token,
			HaConn: 4,
		}
		go srv.Run(bInfo, quickData)
	} else {
		rawConfig, err := parseConfig(configFile)
		if err != nil {
			log.Fatalln("Failed to parse config file: ", err.Error())
		}

		c := rawConfig.Client
		if c != nil {
			if c.Tun != nil && c.Tun.Enable {
				tunName = c.Tun.Name
				if tunName == "" {
					tunName = "cftun0"
					c.Tun.Name = tunName
				}
			}
			c.Run()
		}

		time.Sleep(100 * time.Millisecond)

		s := rawConfig.Server
		if s != nil {
			if s.Token == "quick" {
				isQuick = true
			}
			go s.Run(bInfo, quickData)
		}

	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-sigCh:
			if isQuick {
				quickData.Save()
			}
			if tunName != "" {
				client.DeleteTunDevice(tunName)
			}
			return
		}
	}
}

func printVersion(buildInfo *cliutil.BuildInfo) {
	fmt.Printf("GoOS: %s\nGoArch: %s\nGoVersion: %s\nBuildType: %s\nCftunVersion: %s\nBuildDate: %s\nChecksum: %s\n",
		buildInfo.GoOS, buildInfo.GoArch, buildInfo.GoVersion, buildInfo.BuildType, Version, BuildDate, buildInfo.Checksum)
}
