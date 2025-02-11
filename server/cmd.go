package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"
	"time"

	"github.com/cloudflare/cloudflared/cmd/cloudflared/cliutil"
	"github.com/cloudflare/cloudflared/connection"
	"github.com/cloudflare/cloudflared/orchestration"
	"github.com/cloudflare/cloudflared/signal"
	"github.com/cloudflare/cloudflared/supervisor"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
)

var (
	graceShutdownC chan struct{}
)

func StartServer(
	c *cli.Context,
	namedTunnel *connection.TunnelProperties,
	log *zerolog.Logger,
) error {
	info := &cliutil.BuildInfo{}
	var wg sync.WaitGroup
	errC := make(chan error)

	ctx, cancel := context.WithCancel(c.Context)
	defer cancel()

	connectedSignal := signal.New(make(chan struct{}))
	observer := connection.NewObserver(log, nil)

	tunnelConfig, orchestratorConfig, err := prepareTunnelConfig(ctx, c, info, log, observer, namedTunnel)
	if err != nil {
		log.Err(err).Msg("Couldn't start tunnel")
		return err
	}

	orchestrator, err := orchestration.NewOrchestrator(ctx, orchestratorConfig, tunnelConfig.Tags, nil, tunnelConfig.Log)
	if err != nil {
		return err
	}

	reconnectCh := make(chan supervisor.ReconnectSignal, c.Int("ha-conn"))

	wg.Add(1)
	go func() {
		defer func() {
			wg.Done()
			log.Info().Msg("Tunnel server stopped")
		}()
		errC <- supervisor.StartTunnelDaemon(ctx, tunnelConfig, orchestrator, connectedSignal, reconnectCh, graceShutdownC)
	}()

	gracePeriod, err := gracePeriod(c)
	if err != nil {
		return err
	}
	return waitToShutdown(&wg, cancel, errC, graceShutdownC, gracePeriod, log)
}

func waitToShutdown(wg *sync.WaitGroup,
	cancelServerContext func(),
	errC <-chan error,
	graceShutdownC <-chan struct{},
	gracePeriod time.Duration,
	log *zerolog.Logger,
) error {
	var err error
	select {
	case err = <-errC:
		log.Error().Err(err).Msg("Initiating shutdown")
	case <-graceShutdownC:
		log.Debug().Msg("Graceful shutdown signalled")
		if gracePeriod > 0 {
			// wait for either grace period or service termination
			ticker := time.NewTicker(gracePeriod)
			defer ticker.Stop()
			select {
			case <-ticker.C:
			case <-errC:
			}
		}
	}

	// stop server context
	cancelServerContext()

	// Wait for clean exit, discarding all errors while we wait
	stopDiscarding := make(chan struct{})
	go func() {
		for {
			select {
			case <-errC: // ignore
			case <-stopDiscarding:
				return
			}
		}
	}()
	wg.Wait()
	close(stopDiscarding)

	return err
}

func RunCommand(c *cli.Context) error {
	sc, err := newCommandContext(c)
	if err != nil {
		return err
	}

	if token, err := ParseToken(c.String("token")); err == nil {
		return sc.runWithCredentials(token.Credentials())
	}

	return cliutil.UsageError("Provided Tunnel token is not valid.")
}

func ParseToken(tokenStr string) (*connection.TunnelToken, error) {
	content, err := base64.StdEncoding.DecodeString(tokenStr)
	if err != nil {
		return nil, err
	}

	var token connection.TunnelToken
	if err := json.Unmarshal(content, &token); err != nil {
		return nil, err
	}
	return &token, nil
}
