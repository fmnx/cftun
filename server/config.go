package server

import (
	"github.com/cloudflare/cloudflared/cmd/cloudflared/cliutil"
	"github.com/fmnx/cftun/log"
	"github.com/urfave/cli/v2"
	"os"
)

type Config struct {
	EdgeIPs     []string `yaml:"edge-ips" json:"edge-ips"`
	Token       string   `yaml:"token" json:"token"`
	HaConn      int      `yaml:"ha-conn" json:"ha-conn"`
	BindAddress string   `yaml:"bind-address" json:"bind-address"`
	Warp        *Warp    `yaml:"warp" json:"warp"`
}

func (server *Config) Run(info *cliutil.BuildInfo, quickData *QuickData) {
	buildInfo = info

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

	if server.Warp != nil {
		server.Warp.Run()
	}

	app := &cli.App{}
	app.Flags = []cli.Flag{}

	hostname, _ := os.Hostname()
	app.Commands = []*cli.Command{
		{
			Name:   "run",
			Action: cliutil.ConfiguredAction(RunCommand),
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "token",
					Value: server.Token,
				},
				&cli.StringSliceFlag{
					Name:  "edge",
					Value: cli.NewStringSlice(server.EdgeIPs...),
				},
				&cli.IntFlag{
					Name:  "ha-conn",
					Value: server.HaConn,
				},
				&cli.StringFlag{
					Name:  "bind-address",
					Value: server.BindAddress,
				},
				&cli.StringFlag{
					Name:  "origin-server-name",
					Value: hostname,
				},
				&cli.StringFlag{
					Name:  "service-op-ip",
					Value: "198.41.200.113:80",
				},
				&cli.StringFlag{
					Name:  "management-hostname",
					Value: "management.argotunnel.com",
				}, &cli.BoolFlag{
					Name:  "management-diagnostics",
					Value: true,
				}, &cli.StringFlag{
					Name:  "quickURL",
					Value: quickData.QuickURL,
				}, &cli.StringFlag{
					Name:  "url",
					Value: "",
				},
			},
		},
	}

	_ = app.Run([]string{
		os.Args[0],
		"run",
	})
}
