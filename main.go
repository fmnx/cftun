package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/cloudflare/cloudflared/cmd/cloudflared/cliutil"
	"github.com/fmnx/cftun/client"
	"github.com/fmnx/cftun/log"
	"github.com/fmnx/cftun/server"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
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
	Version            = "unknown"
	BuildDate          = "unknown"
	BuildType          = "DEV"
	CloudflaredVersion = "2025.2.0"
	showVersion        bool
	githubURL          = "https://github.com/fmnx/cftun" // GitHub 地址
)

func init() {
	// 设置 configFile 标志，并指定默认值及说明
	flag.StringVar(&configFile, "C", "./config.json", "")
	flag.StringVar(&token, "T", "", "")
	flag.BoolVar(&showVersion, "V", false, "")

	// 自定义 Usage，显示 version、configFile 和 GitHub 地址
	flag.Usage = func() {
		fmt.Println("Usage:")
		fmt.Printf("  -C\tSpecify the path to the config file.(default: \"./config.json\")\n")
		fmt.Printf("  -T\tWhen a token is provided, the configuration file will be ignored and the program will run in server mode only.\n")
		fmt.Printf("  -V\tDisplay the current binary file version.\n")
		fmt.Println("\nFor more information, visit:", githubURL)
	}
	// 解析命令行参数
	flag.Parse()
}

func main() {
	bInfo := cliutil.GetBuildInfo(BuildType, CloudflaredVersion)
	if showVersion {
		printVersion(bInfo)
		return
	}
	if token != "" {
		svr := &server.Config{
			EdgeIPs:     nil,
			Token:       token,
			HaConn:      4,
			BindAddress: "",
		}
		svr.Run(bInfo)
	} else {
		rawConfig, err := parseConfig(configFile)
		if err != nil {
			log.Fatalln("Failed to parse config file: ", err.Error())
		}

		if rawConfig.Client != nil {
			rawConfig.Client.Run()
		}

		if rawConfig.Server != nil {
			rawConfig.Server.Run(bInfo)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

func printVersion(buildInfo *cliutil.BuildInfo) {
	fmt.Printf("GoOS: %s\nGoArch: %s\nGoVersion: %s\nBuildType: %s\nCftunVersion: %s\nBuildDate: %s\nChecksum: %s\n",
		buildInfo.GoOS, buildInfo.GoArch, buildInfo.GoVersion, buildInfo.BuildType, Version, BuildDate, buildInfo.Checksum)
}
