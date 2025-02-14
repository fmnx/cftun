package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/cloudflare/cloudflared/cmd/cloudflared/cliutil"
	"github.com/cloudflare/cloudflared/config"
	"github.com/cloudflare/cloudflared/connection"
	"github.com/cloudflare/cloudflared/edgediscovery"
	"github.com/cloudflare/cloudflared/edgediscovery/allregions"
	"github.com/cloudflare/cloudflared/features"
	"github.com/cloudflare/cloudflared/ingress"
	"github.com/cloudflare/cloudflared/orchestration"
	"github.com/cloudflare/cloudflared/supervisor"
	"github.com/cloudflare/cloudflared/tlsconfig"
	"github.com/cloudflare/cloudflared/tunnelrpc/pogs"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
	"net"
	"time"
)

func prepareTunnelConfig(
	ctx context.Context,
	c *cli.Context,
	info *cliutil.BuildInfo,
	log *zerolog.Logger,
	observer *connection.Observer,
	namedTunnel *connection.TunnelProperties,
) (*supervisor.TunnelConfig, *orchestration.Config, error) {
	clientID, err := uuid.NewRandom()
	if err != nil {
		return nil, nil, errors.Wrap(err, "can't generate connector UUID")
	}
	log.Info().Msgf("Generated Connector ID: %s", clientID)
	tags := []pogs.Tag{{Name: "ID", Value: clientID.String()}}

	transportProtocol := "quic"
	featureSelector, err := features.NewFeatureSelector(ctx, namedTunnel.Credentials.AccountTag, []string{}, false, log)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to create feature selector")
	}
	clientFeatures := featureSelector.ClientFeatures()
	pqMode := featureSelector.PostQuantumMode()
	if pqMode == features.PostQuantumStrict {
		// Error if the user tries to force a non-quic transport protocol
		if transportProtocol != connection.AutoSelectFlag && transportProtocol != connection.QUIC.String() {
			return nil, nil, fmt.Errorf("post-quantum is only supported with the quic transport")
		}
		transportProtocol = connection.QUIC.String()
	}

	namedTunnel.Client = pogs.ClientInfo{
		ClientID: clientID[:],
		Features: clientFeatures,
		Version:  info.Version(),
		Arch:     info.OSArch(),
	}
	cfg := config.GetConfiguration()
	ingressRules, err := ingress.ParseIngressFromConfigAndCLI(cfg, c, log)
	if err != nil {
		return nil, nil, err
	}

	protocolSelector, err := connection.NewProtocolSelector(transportProtocol, namedTunnel.Credentials.AccountTag, c.IsSet("token"), c.Bool("post-quantum"), edgediscovery.ProtocolPercentage, connection.ResolveTTL, log)
	if err != nil {
		return nil, nil, err
	}
	log.Info().Msgf("Initial protocol %s", protocolSelector.Current())

	edgeTLSConfigs := make(map[connection.Protocol]*tls.Config, len(connection.ProtocolList))
	for _, p := range connection.ProtocolList {
		tlsSettings := p.TLSSettings()
		if tlsSettings == nil {
			return nil, nil, fmt.Errorf("%s has unknown TLS settings", p)
		}
		edgeTLSConfig, err := tlsconfig.CreateTunnelConfig(c, tlsSettings.ServerName)
		if err != nil {
			return nil, nil, errors.Wrap(err, "unable to create TLS config to connect with edge")
		}
		if len(tlsSettings.NextProtos) > 0 {
			edgeTLSConfig.NextProtos = tlsSettings.NextProtos
		}
		edgeTLSConfigs[p] = edgeTLSConfig
	}

	gracePeriod, err := gracePeriod(c)
	if err != nil {
		return nil, nil, err
	}
	edgeIPVersion := allregions.Auto
	edgeBindAddr, err := parseConfigBindAddress(c.String("bind-address"))
	if err != nil {
		return nil, nil, err
	}
	if err := testIPBindable(edgeBindAddr); err != nil {
		return nil, nil, fmt.Errorf("invalid bind-address %s: %v", edgeBindAddr, err)
	}

	tunnelConfig := &supervisor.TunnelConfig{
		GracePeriod:     gracePeriod,
		ReplaceExisting: c.Bool("force"),
		OSArch:          info.OSArch(),
		ClientID:        clientID.String(),
		EdgeAddrs:       c.StringSlice("edge"),
		Region:          c.String("region"),
		EdgeIPVersion:   edgeIPVersion,
		EdgeBindAddr:    edgeBindAddr,
		HAConnections:   c.Int("ha-conn"),
		IsAutoupdated:   false,
		LBPool:          c.String("lb-pool"),
		Tags:            tags,
		Log:             log,
		Observer:        observer,
		ReportedVersion: info.Version(),
		// Note TUN-3758 , we use Int because UInt is not supported with altsrc
		Retries:                             uint(5), // nolint: gosec
		RunFromTerminal:                     true,
		NamedTunnel:                         namedTunnel,
		ProtocolSelector:                    protocolSelector,
		EdgeTLSConfigs:                      edgeTLSConfigs,
		FeatureSelector:                     featureSelector,
		MaxEdgeAddrRetries:                  uint8(8), // nolint: gosec
		RPCTimeout:                          5 * time.Second,
		WriteStreamTimeout:                  0 * time.Second,
		DisableQUICPathMTUDiscovery:         false,
		QUICConnectionLevelFlowControlLimit: 30 * (1 << 20),
		QUICStreamLevelFlowControlLimit:     6 * (1 << 20),
	}

	orchestratorConfig := &orchestration.Config{
		Ingress:            &ingressRules,
		WarpRouting:        ingress.NewWarpRoutingConfig(&cfg.WarpRouting),
		ConfigurationFlags: nil,
		WriteTimeout:       0 * time.Second,
	}
	return tunnelConfig, orchestratorConfig, nil
}

func gracePeriod(c *cli.Context) (time.Duration, error) {
	period := c.Duration("grace-period")
	if period > connection.MaxGracePeriod {
		return time.Duration(0), fmt.Errorf("grace-period must be equal or less than %v", connection.MaxGracePeriod)
	}
	return period, nil
}

func parseConfigBindAddress(ipstr string) (net.IP, error) {
	// Unspecified - it's fine
	if ipstr == "" {
		return nil, nil
	}
	ip := net.ParseIP(ipstr)
	if ip == nil {
		return nil, fmt.Errorf("invalid value for edge-bind-address: %s", ipstr)
	}
	return ip, nil
}

func testIPBindable(ip net.IP) error {
	// "Unspecified" = let OS choose, so always bindable
	if ip == nil {
		return nil
	}

	addr := &net.UDPAddr{IP: ip, Port: 0}
	listener, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	listener.Close()
	return nil
}
