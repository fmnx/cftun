package cfd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
)

type TunnelToken struct {
	AccountTag   string    `json:"a"`
	TunnelSecret []byte    `json:"s"`
	TunnelID     uuid.UUID `json:"t"`
}

type Credentials struct {
	AccountTag   string
	TunnelSecret []byte
	TunnelID     uuid.UUID
}

func (c *Credentials) Auth() TunnelAuth {
	return TunnelAuth{
		AccountTag:   c.AccountTag,
		TunnelSecret: c.TunnelSecret,
	}
}

func (t TunnelToken) Credentials() *Credentials {
	return &Credentials{
		AccountTag:   t.AccountTag,
		TunnelSecret: t.TunnelSecret,
		TunnelID:     t.TunnelID,
	}
}

func ParseToken(tokenStr string) (*TunnelToken, error) {
	content, err := base64.StdEncoding.DecodeString(tokenStr)
	if err != nil {
		return nil, err
	}

	var token TunnelToken
	if err := json.Unmarshal(content, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

func GenerateToken(token *TunnelToken) (string, error) {
	if token == nil {
		return "", fmt.Errorf("token cannot be nil")
	}

	// 将结构体序列化为 JSON
	content, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token: %w", err)
	}

	// 将 JSON 编码为 Base64
	tokenStr := base64.StdEncoding.EncodeToString(content)
	return tokenStr, nil
}
