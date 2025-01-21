package websocket

import (
    "context"
    "encoding/json"
    "log"
    "sync"
    "time"

    "github.com/gorilla/websocket"
    "github.com/sial-ari/solana-token-sniper/internal/db"
    "github.com/sial-ari/solana-token-sniper/internal/models"
)

// Client manages the WebSocket connection and token processing
type Client struct {
    conn          *websocket.Conn
    url           string
    db            *db.Database
    queueSize     int
    mutex         sync.Mutex
    isConnected   bool
    done          chan struct{}
    reconnectWait time.Duration
}

// NewClient creates a new WebSocket client with the specified configuration
func NewClient(url string, database *db.Database, queueSize int) *Client {
    return &Client{
        url:           url,
        db:            database,
        queueSize:     queueSize,
        done:          make(chan struct{}),
        reconnectWait: 5 * time.Second,
    }
}

// Connect establishes the WebSocket connection and handles reconnection
func (c *Client) Connect(ctx context.Context) error {
    dialer := websocket.DefaultDialer
    conn, _, err := dialer.DialContext(ctx, c.url, nil)
    if err != nil {
        return err
    }

    c.mutex.Lock()
    c.conn = conn
    c.isConnected = true
    c.mutex.Unlock()

    // Subscribe to token creation events
    if err := c.subscribeToTokenCreation(); err != nil {
        return err
    }

    // Start message handling in a separate goroutine
    go c.handleMessages(ctx)

    return nil
}

// subscribeToTokenCreation sends the subscription message to the server
func (c *Client) subscribeToTokenCreation() error {
    payload := map[string]interface{}{
        "method": "subscribeNewToken",
    }
    
    message, err := json.Marshal(payload)
    if err != nil {
        return err
    }

    c.mutex.Lock()
    defer c.mutex.Unlock()
    
    return c.conn.WriteMessage(websocket.TextMessage, message)
}

// handleMessages processes incoming WebSocket messages
func (c *Client) handleMessages(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-c.done:
            return
        default:
            _, message, err := c.conn.ReadMessage()
            if err != nil {
                log.Printf("Error reading message: %v", err)
                c.handleDisconnect(ctx)
                return
            }

            if err := c.processMessage(message); err != nil {
                log.Printf("Error processing message: %v", err)
                continue
            }
        }
    }
}

// processMessage handles an individual WebSocket message
func (c *Client) processMessage(message []byte) error {
    var token models.NewToken
    if err := json.Unmarshal(message, &token); err != nil {
        return err
    }

    // Set creation timestamp
    token.CreatedAt = time.Now()

    // Store the token in the database
    if err := c.db.SaveNewToken(&token); err != nil {
        return err
    }

    // Create initial price entry
    initialPrice := &models.TokenPrice{
        Mint:      token.Mint,
        Price:     token.InitialBuy,
        Timestamp: token.CreatedAt,
    }
    
    if err := c.db.SaveTokenPrice(initialPrice); err != nil {
        return err
    }

    // Initialize profit/loss tracking
    profitLoss := &models.TokenProfitLoss{
        Mint:         token.Mint,
        InitialPrice: token.InitialBuy,
        CurrentPrice: token.InitialBuy,
        ProfitLoss:   0,
        ProfitLossPct: 0,
        LastUpdated:   token.CreatedAt,
    }
    
    if err := c.db.UpdateProfitLoss(profitLoss); err != nil {
        return err
    }

    log.Printf("Processed new token: %s (%s)", token.Name, token.Mint)
    return nil
}

// handleDisconnect manages connection loss and reconnection attempts
func (c *Client) handleDisconnect(ctx context.Context) {
    c.mutex.Lock()
    if c.conn != nil {
        c.conn.Close()
    }
    c.isConnected = false
    c.mutex.Unlock()

    // Attempt to reconnect
    ticker := time.NewTicker(c.reconnectWait)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-c.done:
            return
        case <-ticker.C:
            if err := c.Connect(ctx); err != nil {
                log.Printf("Reconnection failed: %v", err)
                continue
            }
            return
        }
    }
}

// Close gracefully shuts down the WebSocket connection
func (c *Client) Close() error {
    c.mutex.Lock()
    defer c.mutex.Unlock()

    close(c.done)
    if c.conn != nil {
        return c.conn.WriteMessage(
            websocket.CloseMessage,
            websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
        )
    }
    return nil
}
