package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/sial-ari/solana-token-sniper/internal/config"
    "github.com/sial-ari/solana-token-sniper/internal/db"
    "github.com/sial-ari/solana-token-sniper/internal/websocket"
    "github.com/sial-ari/solana-token-sniper/internal/jupiter"
)

func main() {
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    database, err := db.Initialize(cfg.DatabasePath)
    if err != nil {
        log.Fatalf("Failed to initialize database: %v", err)
    }

    // Create Jupiter client
    jupiterClient, err := jupiter.NewClient(database, cfg.QueueSize, cfg.QuoteInterval)
    if err != nil {
        log.Fatalf("Failed to create Jupiter client: %v", err)
    }

    wsClient := websocket.NewClient(cfg.WebsocketURL, database, cfg.QueueSize)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Start WebSocket connection
    if err := wsClient.Connect(ctx); err != nil {
        log.Fatalf("Failed to connect to WebSocket server: %v", err)
    }

    // Start price monitoring
    go jupiterClient.StartPriceMonitoring(ctx)

    // Handle shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    <-sigChan
    log.Println("Shutting down...")
    
    jupiterClient.Close()
    if err := wsClient.Close(); err != nil {
        log.Printf("Error closing WebSocket connection: %v", err)
    }
}
