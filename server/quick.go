package server

import (
	"encoding/json"
	"errors"
	"github.com/cloudflare/cloudflared/cmd/cloudflared/cliutil"
	"github.com/cloudflare/cloudflared/connection"
	"github.com/fmnx/cftun/log"
	"github.com/google/uuid"
	"io"
	"net/http"
	"os"
	"time"
)

const httpTimeout = 15 * time.Second
const apiUrl = "https://api.trycloudflare.com/tunnel"

type QuickData struct {
	Token      string    `json:"token"`
	QuickURL   string    `json:"quick-url"`
	LastActive time.Time `json:"last-active"`
}

type QuickTunnelResponse struct {
	Success bool
	Result  QuickTunnel
	Errors  []QuickTunnelError
}

type QuickTunnelError struct {
	Code    int
	Message string
}

type QuickTunnel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Hostname   string `json:"hostname"`
	AccountTag string `json:"account_tag"`
	Secret     []byte `json:"secret"`
}

func (qd *QuickData) Load() error {
	buf, err := os.ReadFile(".quick.json")
	if err != nil {
		return err
	}

	if err = json.Unmarshal(buf, qd); err != nil {
		return err
	}

	if time.Now().After(qd.LastActive.Add(10 * time.Minute)) {
		log.Errorln("\033[31mThe temporary domain name previously applied for has expired and is being reapplied.\033[0m")
		return errors.New("the loaded data has expired")
	}

	return nil
}

func (qd *QuickData) Save() {
	qd.LastActive = time.Now()
	// 将内存中的数据静态化
	updateFile, err := json.MarshalIndent(qd, "", "  ")
	if err != nil {
		log.Errorln("Error generating JSON:", err)
		return
	}
	err = os.WriteFile(".quick.json", updateFile, 0644)
	if err != nil {
		log.Errorln("Error writing config file:", err)
		return
	}
}

func ApplyQuickURL(buildInfo *cliutil.BuildInfo) (string, string, error) {
	client := http.Client{
		Transport: &http.Transport{
			TLSHandshakeTimeout:   httpTimeout,
			ResponseHeaderTimeout: httpTimeout,
		},
		Timeout: httpTimeout,
	}

	req, err := http.NewRequest(http.MethodPost, apiUrl, nil)

	if err != nil {
		return "", "", errors.New("failed to build quick tunnel request")
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", buildInfo.UserAgent())
	resp, err := client.Do(req)

	if err != nil {
		return "", "", errors.New("failed to request quick Tunnel")
	}
	defer resp.Body.Close()

	rspBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", errors.New("failed to read quick-tunnel response")
	}
	var data QuickTunnelResponse
	if err := json.Unmarshal(rspBody, &data); err != nil {
		return "", "", errors.New("failed to unmarshal quick Tunnel")
	}

	tunnelID, err := uuid.Parse(data.Result.ID)
	if err != nil {
		return "", "", errors.New("failed to parse quick Tunnel ID")
	}

	tunnelToken := connection.TunnelToken{
		AccountTag:   data.Result.AccountTag,
		TunnelSecret: data.Result.Secret,
		TunnelID:     tunnelID,
	}

	token, err := GenerateToken(&tunnelToken)
	if err != nil {
		return "", "", err
	}

	return token, data.Result.Hostname, nil
}
