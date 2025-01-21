package jupiter

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"

    "github.com/sial-ari/solana-token-sniper/internal/db"
    "github.com/sial-ari/solana-token-sniper/internal/models"
    "github.com/ilkamo/jupiter-go/jupiter"
)

// Client manages Jupiter API interactions and price monitoring
type Client struct {
    jupClient    *jupiter.Client
    db           *db.Database
    queueSize    int
    interval     time.Duration
    mutex        sync.RWMutex
    monitoredTokens map[string]bool
    done         chan struct{}
}

// NewClient creates a new Jupiter client instance
func NewClient(database *db.Database, queueSize int, interval int) (*Client, error) {
    jupClient, err := jupiter.NewClientWithResponses(jupiter.DefaultAPIURL)
    if err != nil {
        return nil, fmt.Errorf("failed to create Jupiter client: %w", err)
    }

    return &Client{
        jupClient:       jupClient,
        db:             database,
        queueSize:      queueSize,
        interval:       time.Duration(interval) * time.Second,
        monitoredTokens: make(map[string]bool),
        done:           make(chan struct{}),
    }, nil
}

// StartPriceMonitoring begins monitoring prices for tokens in the queue
func (c *Client) StartPriceMonitoring(ctx context.Context) {
    ticker := time.NewTicker(c.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-c.done:
            return
        case <-ticker.C:
            if err := c.updatePrices(ctx); err != nil {
                log.Printf("Error updating prices: %v", err)
            }
        }
    }
}

// updatePrices fetches current prices for all monitored tokens
func (c *Client) updatePrices(ctx context.Context) error {
    tokens, err := c.db.GetTokensInQueue(c.queueSize)
    if err != nil {
        return fmt.Errorf("failed to get tokens from queue: %w", err)
    }

    for _, token := range tokens {
        // Skip if we're shutting down
        select {
        case <-ctx.Done():
            return nil
        default:
        }

        quote, err := c.getQuote(ctx, token.Mint)
        if err != nil {
            log.Printf("Error getting quote for %s: %v", token.Mint, err)
            continue
        }

        // Save the new price point
        pricePoint := &models.TokenPrice{
            Mint:      token.Mint,
            Price:     quote.Price,
            Timestamp: time.Now(),
        }
        
        if err := c.db.SaveTokenPrice(pricePoint); err != nil {
            log.Printf("Error saving price for %s: %v", token.Mint, err)
            continue
        }

        // Update profit/loss calculations
        if err := c.updateProfitLoss(ctx, token.Mint); err != nil {
            log.Printf("Error updating P&L for %s: %v", token.Mint, err)
            continue
        }
    }

    return nil
}

// getQuote retrieves the current price quote for a token
func (c *Client) getQuote(ctx context.Context, outputMint string) (*jupiter.QuoteResponse, error) {
    slippageBps := 250
    inputAmount := int64(100000) // 0.0001 SOL for price check

    response, err := c.jupClient.GetQuoteWithResponse(ctx, &jupiter.GetQuoteParams{
        InputMint:   "So11111111111111111111111111111111111111112", // SOL
        OutputMint:  outputMint,
        Amount:      inputAmount,
        SlippageBps: &slippageBps,
    })
    
    if err != nil {
        return nil, fmt.Errorf("failed to get quote: %w", err)
    }

    if response.JSON200 == nil {
        return nil, fmt.Errorf("no valid quote response received")
    }

    return response.JSON200, nil
}

// updateProfitLoss calculates and stores current P&L for a token
func (c *Client) updateProfitLoss(ctx context.Context, mint string) error {
    prices, err := c.db.GetPriceHistory(mint)
    if err != nil || len(prices) < 2 {
        return fmt.Errorf("failed to get price history: %w", err)
    }

    initialPrice := prices[len(prices)-1].Price  // First recorded price
    currentPrice := prices[0].Price              // Most recent price
    
    profitLoss := currentPrice - initialPrice
    profitLossPct := (profitLoss / initialPrice) * 100

    pl := &models.TokenProfitLoss{
        Mint:          mint,
        InitialPrice:  initialPrice,
        CurrentPrice:  currentPrice,
        ProfitLoss:    profitLoss,
        ProfitLossPct: profitLossPct,
        LastUpdated:   time.Now(),
    }

    return c.db.UpdateProfitLoss(pl)
}

// ExecuteSwap performs a token swap using Jupiter
func (c *Client) ExecuteSwap(ctx context.Context, mint string, solAmount float64, userPubKey string) error {
    // Convert SOL amount to lamports
    lamports := int64(solAmount * 1e9)
    
    // Get quote for the swap
    slippageBps := 250
    response, err := c.jupClient.GetQuoteWithResponse(ctx, &jupiter.GetQuoteParams{
        InputMint:   "So11111111111111111111111111111111111111112",
        OutputMint:  mint,
        Amount:      lamports,
        SlippageBps: &slippageBps,
    })

    if err != nil {
        return fmt.Errorf("failed to get swap quote: %w", err)
    }

    if response.JSON200 == nil {
        return fmt.Errorf("no valid quote response received")
    }

    // Set up swap parameters
    prioritizationFeeLamports := jupiter.SwapRequest_PrioritizationFeeLamports{}
    if err = prioritizationFeeLamports.UnmarshalJSON([]byte(`"auto"`)); err != nil {
        return fmt.Errorf("error setting prioritization fee: %w", err)
    }

    dynamicComputeUnitLimit := true

    // Execute the swap
    swapResponse, err := c.jupClient.PostSwapWithResponse(ctx, jupiter.PostSwapJSONRequestBody{
        QuoteResponse:             *response.JSON200,
        UserPublicKey:            userPubKey,
        PrioritizationFeeLamports: &prioritizationFeeLamports,
        DynamicComputeUnitLimit:   &dynamicComputeUnitLimit,
    })

    if err != nil {
        return fmt.Errorf("failed to execute swap: %w", err)
    }

    if swapResponse.JSON200 == nil {
        return fmt.Errorf("no valid swap response received")
    }

    // Log successful swap
    log.Printf("Successful swap for token %s, amount: %f SOL", mint, solAmount)
    return nil
}

// Close stops the price monitoring
func (c *Client) Close() {
    close(c.done)
}
