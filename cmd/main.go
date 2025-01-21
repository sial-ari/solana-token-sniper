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
)

func main() {
    // Load configuration
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Initialize database
    database, err := db.Initialize(cfg.DatabasePath)
    if err != nil {
        log.Fatalf("Failed to initialize database: %v", err)
    }

    // Create WebSocket client
    wsClient := websocket.NewClient(cfg.WebsocketURL, database, cfg.QueueSize)

    // Set up context with cancellation
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Handle graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    // Connect to WebSocket server
    if err := wsClient.Connect(ctx); err != nil {
        log.Fatalf("Failed to connect to WebSocket server: %v", err)
    }

    // Wait for shutdown signal
    <-sigChan
    log.Println("Shutting down...")
    
    // Close WebSocket connection
    if err := wsClient.Close(); err != nil {
        log.Printf("Error closing WebSocket connection: %v", err)
    }
}
