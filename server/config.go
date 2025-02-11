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
}

func (server *Config) Run() {
	app := &cli.App{}
	app.Flags = []cli.Flag{}
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
			},
		},
	}

	err := app.Run([]string{
		os.Args[0],
		"run",
	})
	if err != nil {
		log.Errorln(err.Error())
		return
	}
}
