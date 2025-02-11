package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/cloudflare/cloudflared/edgediscovery"
	"github.com/cloudflare/cloudflared/ingress"
	"github.com/cloudflare/cloudflared/logger"
	"github.com/cloudflare/cloudflared/management"
	"github.com/google/uuid"
	"sync"
	"time"

	"github.com/cloudflare/cloudflared/cmd/cloudflared/cliutil"
	cfdflags "github.com/cloudflare/cloudflared/cmd/cloudflared/flags"
	"github.com/cloudflare/cloudflared/connection"
	"github.com/cloudflare/cloudflared/orchestration"
	"github.com/cloudflare/cloudflared/signal"
	"github.com/cloudflare/cloudflared/supervisor"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
)

var (
	graceShutdownC chan struct{}
	buildInfo      *cliutil.BuildInfo
)

func StartServer(
	c *cli.Context,
	namedTunnel *connection.TunnelProperties,
	log *zerolog.Logger,
) error {
	var wg sync.WaitGroup
	errC := make(chan error)

	ctx, cancel := context.WithCancel(c.Context)
	defer cancel()

	connectedSignal := signal.New(make(chan struct{}))
	observer := connection.NewObserver(log, nil)

	tunnelConfig, orchestratorConfig, err := prepareTunnelConfig(ctx, c, buildInfo, log, observer, namedTunnel)
	if err != nil {
		log.Err(err).Msg("Couldn't start tunnel")
		return err
	}

	serviceIP := c.String("service-op-ip")
	if edgeAddrs, err := edgediscovery.ResolveEdge(log, tunnelConfig.Region, tunnelConfig.EdgeIPVersion); err == nil {
		if serviceAddr, err := edgeAddrs.GetAddrForRPC(); err == nil {
			serviceIP = serviceAddr.TCP.String()
		}
	}

	var clientID uuid.UUID
	if tunnelConfig.NamedTunnel != nil {
		clientID, err = uuid.FromBytes(tunnelConfig.NamedTunnel.Client.ClientID)
		if err != nil {
			// set to nil for classic tunnels
			clientID = uuid.Nil
		}
	}

	mgmt := management.New(
		c.String("management-hostname"),
		c.Bool("management-diagnostics"),
		serviceIP,
		clientID,
		c.String(cfdflags.ConnectorLabel),
		logger.ManagementLogger.Log,
		logger.ManagementLogger,
	)
	internalRules := []ingress.Rule{ingress.NewManagementRule(mgmt)}

	orchestrator, err := orchestration.NewOrchestrator(ctx, orchestratorConfig, tunnelConfig.Tags, internalRules, tunnelConfig.Log)
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

	return errors.New("provided Tunnel token is not valid")
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
