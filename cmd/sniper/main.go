// cmd/sniper/main.go
package main

import (
    "context"
    "flag"
    "os"
    "os/signal"
    "syscall"

    "github.com/yourusername/solana-token-sniper/internal/scanner"
    "github.com/yourusername/solana-token-sniper/internal/metrics"
    "github.com/yourusername/solana-token-sniper/internal/logger"
)

func main() {
    // Parse command line flags
    configPath := flag.String("config", "config.json", "Path to configuration file")
    flag.Parse()

    // Initialize logger
    log, err := logger.NewLogger("logs/sniper.log")
    if err != nil {
        panic(fmt.Sprintf("Failed to initialize logger: %v", err))
    }
    defer log.Close()

    // Initialize metrics client
    metrics, err := metrics.NewMetricsClient(
        context.Background(),
        os.Getenv("DATABASE_URL"),
        log,
    )
    if err != nil {
        log.Error(fmt.Sprintf("Failed to initialize metrics client: %v", err))
        os.Exit(1)
    }
    defer metrics.Close()

    // Initialize scanner
    tokenScanner := scanner.NewScanner(metrics, log)

    // Start the scanner
    if err := tokenScanner.Start(); err != nil {
        log.Error(fmt.Sprintf("Failed to start scanner: %v", err))
        os.Exit(1)
    }

    // Handle shutdown signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    // Wait for shutdown signal
    <-sigChan
    log.Info("Shutdown signal received, stopping scanner...")
    
    // Graceful shutdown
    tokenScanner.Stop()
}
