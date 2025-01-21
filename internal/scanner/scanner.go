// internal/scanner/scanner.go
package scanner

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"
    "github.com/gorilla/websocket"
    "github.com/sial-ari/solana-token-sniper/internal/logger"
)

// NewToken represents the structure of a newly created token event from pumpportal.fun
type NewToken struct {
    BondingCurveKey       string  `json:"bondingCurveKey"`
    InitialBuy            float64 `json:"initialBuy"`
    MarketCapSol          float64 `json:"marketCapSol"`
    Mint                  string  `json:"mint"`
    Name                  string  `json:"name"`
    Signature             string  `json:"signature"`
    SolAmount             float64 `json:"solAmount"`
    Symbol                string  `json:"symbol"`
    TraderPublicKey       string  `json:"traderPublicKey"`
    TxType                string  `json:"txType"`
    URI                   string  `json:"uri"`
    VSolInBondingCurve    float64 `json:"vSolInBondingCurve"`
    VTokensInBondingCurve float64 `json:"vTokensInBondingCurve"`
}

// TokenMetrics represents derived metrics for token analysis
type TokenMetrics struct {
    TokenPrice       float64   // Calculated price in SOL
    LiquidityRatio   float64   // Ratio of SOL to token liquidity
    MarketDepth      float64   // Measure of market depth based on bonding curve
    CreationTime     time.Time // Time when token was first detected
}

// Scanner manages the WebSocket connection and token event processing
type Scanner struct {
    conn          *websocket.Conn
	logger        *logger.Logger
    mutex         sync.RWMutex
    isRunning     bool
    ctx           context.Context
    cancel        context.CancelFunc
    reconnectCh   chan struct{}
    seenTokens    map[string]TokenMetrics // Track tokens we've already processed
    minMarketCap  float64                 // Minimum market cap threshold in SOL
    minLiquidity  float64                 // Minimum liquidity threshold in SOL
}

func NewScanner(logger *logger.Logger, minMarketCap, minLiquidity float64) *Scanner {
    ctx, cancel := context.WithCancel(context.Background())
    return &Scanner{
		logger:       logger,
        ctx:          ctx,
        cancel:       cancel,
        reconnectCh:  make(chan struct{}, 1),
        seenTokens:   make(map[string]TokenMetrics),
        minMarketCap: minMarketCap,
        minLiquidity: minLiquidity,
    }
}

// calculateTokenMetrics calculates various metrics from the token event
func (s *Scanner) calculateTokenMetrics(token *NewToken) TokenMetrics {
    var metrics TokenMetrics

    // Calculate token price: SOL in bonding curve / tokens in bonding curve
    if token.VTokensInBondingCurve > 0 {
        metrics.TokenPrice = token.VSolInBondingCurve / token.VTokensInBondingCurve
    }

    // Calculate liquidity ratio: SOL in bonding curve / market cap
    if token.MarketCapSol > 0 {
        metrics.LiquidityRatio = token.VSolInBondingCurve / token.MarketCapSol
    }

    // Calculate market depth based on bonding curve volumes
    metrics.MarketDepth = token.VSolInBondingCurve * token.VTokensInBondingCurve
    metrics.CreationTime = time.Now()

    return metrics
}

// processMessage handles the incoming token event message
func (s *Scanner) processMessage(message []byte) error {
    start := time.Now()

    // Parse the event
    var token NewToken
    if err := json.Unmarshal(message, &token); err != nil {
        return fmt.Errorf("failed to unmarshal token event: %w", err)
    }

    // Skip if we've already seen this token
    s.mutex.Lock()
    if _, exists := s.seenTokens[token.Mint]; exists {
        s.mutex.Unlock()
        return nil
    }

    // Calculate token metrics
    metrics := s.calculateTokenMetrics(&token)
    s.seenTokens[token.Mint] = metrics
    s.mutex.Unlock()

    // Apply filters for token quality
    if !s.isTokenQualified(&token, metrics) {
        s.logger.Info(fmt.Sprintf("Token %s (%s) filtered out: below thresholds", token.Name, token.Mint))
        return nil
    }

    // Record the token event in the metrics database
    err := s.metrics.RecordTokenPrice(s.ctx, metrics.TokenPrice{
        Address:   token.Mint,
        Symbol:    token.Symbol,
        Price:     metrics.TokenPrice,
        Timestamp: time.Now(),
        Details: map[string]interface{}{
            "market_cap":        token.MarketCapSol,
            "liquidity":         token.VSolInBondingCurve,
            "initial_buy":       token.InitialBuy,
            "bonding_curve_key": token.BondingCurveKey,
            "trader":            token.TraderPublicKey,
            "tx_type":           token.TxType,
        },
    })

    if err != nil {
        return fmt.Errorf("failed to record token event: %w", err)
    }

    s.logger.Info(fmt.Sprintf("New qualified token detected: %s (%s) - Price: %f SOL, Market Cap: %f SOL, Liquidity: %f SOL",
        token.Name, token.Mint, metrics.TokenPrice, token.MarketCapSol, token.VSolInBondingCurve))

    // Record processing time metric
    s.recordMetric("token_processing", start, true, map[string]string{
        "mint":       token.Mint,
        "tx_type":    token.TxType,
        "market_cap": fmt.Sprintf("%.2f", token.MarketCapSol),
    })

    return nil
}

// isTokenQualified checks if the token meets our quality criteria
func (s *Scanner) isTokenQualified(token *NewToken, metrics TokenMetrics) bool {
    // Check market cap threshold
    if token.MarketCapSol < s.minMarketCap {
        return false
    }

    // Check liquidity threshold
    if token.VSolInBondingCurve < s.minLiquidity {
        return false
    }

    // Check for suspicious token characteristics
    if metrics.LiquidityRatio < 0.1 { // Less than 10% liquidity ratio is suspicious
        return false
    }

    // Additional checks can be added here based on:
    // - Token name and symbol validation
    // - Trader public key reputation
    // - Bonding curve analysis
    // - Transaction type verification

    return true
}

func (s *Scanner) Start() error {
    s.mutex.Lock()
    if s.isRunning {
        s.mutex.Unlock()
        return fmt.Errorf("scanner is already running")
    }
    s.isRunning = true
    s.mutex.Unlock()

    // Start the main processing loop
    go s.processLoop()

    return nil
}

func (s *Scanner) Stop() {
    s.cancel()
    s.mutex.Lock()
    s.isRunning = false
    s.mutex.Unlock()

    if s.conn != nil {
        s.conn.Close()
    }
}

func (s *Scanner) processLoop() {
    for {
        select {
        case <-s.ctx.Done():
            return
        default:
            if err := s.connect(); err != nil {
                s.logger.Error(fmt.Sprintf("Connection error: %v", err))
                s.scheduleReconnect()
                continue
            }

            s.handleMessages()
        }
    }
}

func (s *Scanner) connect() error {
    start := time.Now()
    uri := "wss://pumpportal.fun/api/data"
    
    conn, _, err := websocket.DefaultDialer.Dial(uri, nil)
    if err != nil {
        s.recordMetric("websocket_connection", start, false, map[string]string{
            "error": err.Error(),
        })
        return fmt.Errorf("failed to connect to WebSocket: %w", err)
    }

    s.conn = conn
    s.recordMetric("websocket_connection", start, true, nil)

    // Subscribe to token creation events
    if err := s.subscribe(); err != nil {
        conn.Close()
        return fmt.Errorf("failed to subscribe: %w", err)
    }

    return nil
}

func (s *Scanner) subscribe() error {
    start := time.Now()
    payload := map[string]interface{}{
        "method": "subscribeNewToken",
    }

    message, err := json.Marshal(payload)
    if err != nil {
        s.recordMetric("subscription", start, false, map[string]string{
            "error": "marshal_failed",
        })
        return fmt.Errorf("failed to marshal subscription payload: %w", err)
    }

    err = s.conn.WriteMessage(websocket.TextMessage, message)
    if err != nil {
        s.recordMetric("subscription", start, false, map[string]string{
            "error": "write_failed",
        })
        return fmt.Errorf("failed to send subscription message: %w", err)
    }

    s.recordMetric("subscription", start, true, nil)
    s.logger.Info("Successfully subscribed to new token creation events")
    return nil
}

func (s *Scanner) handleMessages() {
    for {
        select {
        case <-s.ctx.Done():
            return
        default:
            start := time.Now()
            _, message, err := s.conn.ReadMessage()
            if err != nil {
                s.recordMetric("message_read", start, false, map[string]string{
                    "error": err.Error(),
                })
                s.logger.Error(fmt.Sprintf("Error reading message: %v", err))
                s.scheduleReconnect()
                return
            }

            // Process the message
            if err := s.processMessage(message); err != nil {
                s.recordMetric("message_processing", start, false, map[string]string{
                    "error": err.Error(),
                })
                s.logger.Error(fmt.Sprintf("Error processing message: %v", err))
                continue
            }

            s.recordMetric("message_processing", start, true, nil)
        }
    }
}

func (s *Scanner) processMessage(message []byte) error {
    var event TokenEvent
    if err := json.Unmarshal(message, &event); err != nil {
        return fmt.Errorf("failed to unmarshal token event: %w", err)
    }

    // Record the token creation event in the metrics database
    err := s.metrics.RecordTokenPrice(s.ctx, metrics.TokenPrice{
        Address:   event.Address,
        Symbol:    event.Symbol,
        Price:     0, // Initial price will be updated by price checker
        Timestamp: time.Now(),
    })

    if err != nil {
        return fmt.Errorf("failed to record token event: %w", err)
    }

    s.logger.Info(fmt.Sprintf("New token detected: %s (%s)", event.Name, event.Address))
    return nil
}

func (s *Scanner) scheduleReconnect() {
    select {
    case s.reconnectCh <- struct{}{}:
        time.Sleep(5 * time.Second) // Basic backoff
    default:
        // Reconnection already scheduled
    }
}

func (s *Scanner) recordMetric(operation string, startTime time.Time, success bool, details map[string]string) {
    elapsed := time.Since(startTime)
    err := s.metrics.RecordPerformance(s.ctx, metrics.PerformanceMetric{
        Operation:  operation,
        Duration:   elapsed,
        Success:    success,
        Timestamp:  time.Now(),
        Details:    details,
    })

    if err != nil {
        s.logger.Error(fmt.Sprintf("Failed to record metric: %v", err))
    }
