package server

import (
	"fmt"
	"github.com/fmnx/cftun/log"
	"github.com/fmnx/cftun/server/cfd"
	"github.com/google/uuid"
	"net"
	"net/netip"
	"runtime"
)

type BuildInfo struct {
	GoOS               string `json:"go_os"`
	GoVersion          string `json:"go_version"`
	GoArch             string `json:"go_arch"`
	BuildType          string `json:"build_type"`
	CloudflaredVersion string `json:"cloudflared_version"`
}

func GetBuildInfo(buildType, version string) *BuildInfo {
	return &BuildInfo{
		GoOS:               runtime.GOOS,
		GoVersion:          runtime.Version(),
		GoArch:             runtime.GOARCH,
		BuildType:          buildType,
		CloudflaredVersion: version,
	}
}

func (bi *BuildInfo) UserAgent() string {
	return fmt.Sprintf("cloudflared/%s", bi.CloudflaredVersion)
}

type Config struct {
	EdgeIPs     []string `yaml:"edge-ips" json:"edge-ips"`
	Token       string   `yaml:"token" json:"token"`
	HaConn      int      `yaml:"ha-conn" json:"ha-conn"`
	BindAddress string   `yaml:"bind-address" json:"bind-address"`
	Warp        *Warp    `yaml:"warp" json:"warp"`
}

func (server *Config) Run(info *BuildInfo, quickData *QuickData) {
	if server.HaConn == 0 {
		server.HaConn = 4
	}

	if server.Token == "quick" {
		if err := quickData.Load(); err != nil {
			quickData.Token, quickData.QuickURL, err = ApplyQuickURL(info)
			if err != nil {
				log.Fatalln(err.Error())
			}
		}
		server.Token = quickData.Token
		log.Infoln("\033[36mTHE TEMPORARY DOMAIN YOU HAVE APPLIED FOR IS: \033[0m%s", quickData.QuickURL)
	}

	dialFunc := net.Dial
	var proxy4, proxy6 bool
	if server.Warp != nil && (server.Warp.Proxy4 || server.Warp.Proxy6) {
		dialFunc = server.Warp.Run()
		proxy4, proxy6 = server.Warp.Proxy4, server.Warp.Proxy6
	}

	clientID, _ := uuid.NewRandom()
	var edgeIPS chan netip.AddrPort
	if len(server.EdgeIPs) > 0 {
		edgeIPS = make(chan netip.AddrPort, len(server.EdgeIPs))
		for _, addr := range server.EdgeIPs {
			edgeAddr, err := netip.ParseAddrPort(addr)
			if err == nil {
				edgeIPS <- edgeAddr
			}
		}
	}

	edgeTunnel := cfd.EdgeTunnelServer{
		Token:        server.Token,
		HaConn:       server.HaConn,
		EdgeIPS:      edgeIPS,
		EdgeBindAddr: net.ParseIP(server.BindAddress),
		Proxy: &cfd.Proxy{
			DialFunc: dialFunc,
			Proxy4:   proxy4,
			Proxy6:   proxy6,
		},
		ClientInfo: &cfd.ClientInfo{
			ClientID: clientID[:],
			Version:  info.CloudflaredVersion,
			Arch:     info.GoArch,
		},
	}
	for i := 0; i < server.HaConn; i++ {
		go func(i int) {
			err := edgeTunnel.Serve(i)
			if err != nil {
				log.Errorln(err.Error())
			}
		}(i)
	}
}
