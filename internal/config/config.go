// internal/config/config.go
package config

import (
	"os"
	"encoding/json"
)

type Config struct {
	// Solana configuration
	SolanaRPC string `json:"solana_rpc"`

	// Jupiter configuration
	JupiterAPIEndpoint string `json:"jupiter_api_endpoint"`

	// Database configuration
	DatabasePath string `json:"database_path"`

	// Trading configuration
	WalletPrivateKey string `json:"wallet_private_key"`
	DryRun           bool   `json:"dry_run"`

	// Scanning configuration
	ScanInterval     int    `json:"scan_interval"`
	ProfitThreshold  float64 `json:"profit_threshold"`

	// Telegram configuration
	TelegramToken    string `json:"telegram_token"`
	TelegramChatID   int64  `json:"telegram_chat_id"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *Config) IsDryRun() bool {
	return c.DryRun
}
