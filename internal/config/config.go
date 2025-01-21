package config

import (
	"os"
	"strconv"
)

type Config struct {
	WebsocketURL     string
	QueueSize        int
	QuoteInterval    int
	DatabasePath     string
	TelegramToken    string
	SolanaRPCURL     string
	DryRun           bool
}

func LoadConfig() (*Config, error) {
	queueSize, _ := strconv.Atoi(getEnvWithDefault("QUEUE_SIZE", "5"))
	quoteInterval, _ := strconv.Atoi(getEnvWithDefault("QUOTE_INTERVAL", "30"))

	return &Config{
		WebsocketURL:  getEnvWithDefault("WEBSOCKET_URL", "wss://pumpportal.fun/api/data"),
		QueueSize:     queueSize,
		QuoteInterval: quoteInterval,
		DatabasePath:  getEnvWithDefault("DATABASE_PATH", "tokens.db"),
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
		SolanaRPCURL:  getEnvWithDefault("SOLANA_RPC_URL", "https://api.mainnet-beta.solana.com"),
		DryRun:        os.Getenv("DRY_RUN") == "true",
	}, nil
}

func getEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
