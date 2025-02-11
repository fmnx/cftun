package server

import (
	"github.com/cloudflare/cloudflared/connection"
	"github.com/cloudflare/cloudflared/logger"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
)

type subcommandContext struct {
	c   *cli.Context
	log *zerolog.Logger
}

func newCommandContext(c *cli.Context) (*subcommandContext, error) {
	return &subcommandContext{
		c:   c,
		log: logger.CreateLoggerFromContext(c, logger.EnableTerminalLog),
	}, nil
}

func (sc *subcommandContext) runWithCredentials(credentials connection.Credentials) error {
	sc.log.Info().Str("tunnelID", credentials.TunnelID.String()).Msg("Starting tunnel")

	return StartServer(
		sc.c,
		&connection.TunnelProperties{Credentials: credentials},
		sc.log,
	)
}
