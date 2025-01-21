package telegram

import (
    "context"
    "fmt"
    "log"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/sial-ari/solana-token-sniper/internal/config"
    "github.com/sial-ari/solana-token-sniper/internal/db"
    "github.com/sial-ari/solana-token-sniper/internal/jupiter"
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
    api          *tgbotapi.BotAPI
    db           *db.Database
    jupiter      *jupiter.Client
    config       *config.Config
    allowedUsers map[int64]bool
    mutex        sync.RWMutex
}

func NewBot(token string, database *db.Database, jupiterClient *jupiter.Client, cfg *config.Config) (*Bot, error) {
    api, err := tgbotapi.NewBotAPI(token)
    if err != nil {
        return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
    }

    // For development, you might want to enable debugging
    api.Debug = true

    return &Bot{
        api:          api,
        db:           database,
        jupiter:      jupiterClient,
        config:       cfg,
        allowedUsers: make(map[int64]bool),
    }, nil
}

func (b *Bot) Start(ctx context.Context) error {
    updateConfig := tgbotapi.NewUpdate(0)
    updateConfig.Timeout = 60

    updates := b.api.GetUpdatesChan(updateConfig)

    // Start processing updates
    for {
        select {
        case <-ctx.Done():
            return nil
        case update := <-updates:
            if update.Message == nil {
                continue
            }

            // Process the message in a separate goroutine
            go b.handleMessage(ctx, update.Message)
        }
    }
}

func (b *Bot) handleMessage(ctx context.Context, message *tgbotapi.Message) {
    // Split command and arguments
    parts := strings.Fields(message.Text)
    if len(parts) == 0 {
        return
    }

    command := strings.ToLower(parts[0])
    args := parts[1:]

    var reply string
    var err error

    switch command {
    case "/start":
        reply = "Welcome to the Solana Token Sniper Bot!\n\n" +
                "Available commands:\n" +
                "/tokens - View tokens in queue\n" +
                "/price <symbol> - Get current price for a token\n" +
                "/pl - View profit/loss for all monitored tokens\n" +
                "/config - View current configuration\n" +
                "/setconfig <key> <value> - Update configuration\n" +
                "/dryrun <mint> <amount> - Simulate a token swap\n" +
                "/swap <mint> <amount> - Execute a real token swap"

    case "/tokens":
        reply, err = b.handleTokensCommand(ctx)

    case "/price":
        if len(args) < 1 {
            reply = "Usage: /price <symbol>"
        } else {
            reply, err = b.handlePriceCommand(ctx, args[0])
        }

    case "/pl":
        reply, err = b.handleProfitLossCommand(ctx)

    case "/config":
        reply = b.handleConfigCommand()

    case "/setconfig":
        if len(args) < 2 {
            reply = "Usage: /setconfig <key> <value>"
        } else {
            reply, err = b.handleSetConfigCommand(args[0], args[1])
        }

    case "/dryrun":
        if len(args) < 2 {
            reply = "Usage: /dryrun <mint> <amount>"
        } else {
            reply, err = b.handleDryRunCommand(ctx, args[0], args[1])
        }

    case "/swap":
        if len(args) < 2 {
            reply = "Usage: /swap <mint> <amount>"
        } else {
            reply, err = b.handleSwapCommand(ctx, args[0], args[1])
        }
    }

    if err != nil {
        reply = fmt.Sprintf("Error: %v", err)
    }

    // Send the reply
    msg := tgbotapi.NewMessage(message.Chat.ID, reply)
    msg.ParseMode = tgbotapi.ModeMarkdown
    
    if _, err := b.api.Send(msg); err != nil {
        log.Printf("Error sending message: %v", err)
    }
}

func (b *Bot) handleTokensCommand(ctx context.Context) (string, error) {
    tokens, err := b.db.GetTokensInQueue(b.config.QueueSize)
    if err != nil {
        return "", err
    }

    if len(tokens) == 0 {
        return "No tokens currently in queue", nil
    }

    var sb strings.Builder
    sb.WriteString("*Current Token Queue:*\n\n")

    for i, token := range tokens {
        sb.WriteString(fmt.Sprintf("%d. *%s* (%s)\n", i+1, token.Name, token.Symbol))
        sb.WriteString(fmt.Sprintf("   Mint: `%s`\n", token.Mint))
        sb.WriteString(fmt.Sprintf("   Initial Price: %.8f SOL\n", token.InitialBuy))
        sb.WriteString(fmt.Sprintf("   Market Cap: %.2f SOL\n", token.MarketCapSol))
        sb.WriteString(fmt.Sprintf("   Created: %s\n\n", token.CreatedAt.Format(time.RFC822)))
    }

    return sb.String(), nil
}

func (b *Bot) handlePriceCommand(ctx context.Context, symbol string) (string, error) {
    // Get token details from database
    token, err := b.db.GetTokenBySymbol(symbol)
    if err != nil {
        return "", fmt.Errorf("token not found: %s", symbol)
    }

    // Get latest price from price history
    prices, err := b.db.GetPriceHistory(token.Mint)
    if err != nil || len(prices) == 0 {
        return "", fmt.Errorf("no price data available for %s", symbol)
    }

    latestPrice := prices[0]
    
    return fmt.Sprintf("*%s (%s)*\n"+
        "Current Price: %.8f SOL\n"+
        "Last Updated: %s",
        token.Name,
        token.Symbol,
        latestPrice.Price,
        latestPrice.Timestamp.Format(time.RFC822)), nil
}

func (b *Bot) handleProfitLossCommand(ctx context.Context) (string, error) {
    tokens, err := b.db.GetTokensInQueue(b.config.QueueSize)
    if err != nil {
        return "", err
    }

    var sb strings.Builder
    sb.WriteString("*Profit/Loss Summary:*\n\n")

    for _, token := range tokens {
        pl, err := b.db.GetProfitLoss(token.Mint)
        if err != nil {
            continue
        }

        sb.WriteString(fmt.Sprintf("*%s (%s)*\n", token.Name, token.Symbol))
        sb.WriteString(fmt.Sprintf("Initial: %.8f SOL\n", pl.InitialPrice))
        sb.WriteString(fmt.Sprintf("Current: %.8f SOL\n", pl.CurrentPrice))
        sb.WriteString(fmt.Sprintf("P/L: %.2f%% (%.8f SOL)\n\n", 
            pl.ProfitLossPct, pl.ProfitLoss))
    }

    return sb.String(), nil
}

func (b *Bot) handleConfigCommand() string {
    return fmt.Sprintf(
        "*Current Configuration:*\n"+
            "Queue Size: %d\n"+
            "Quote Interval: %d seconds\n"+
            "Dry Run Mode: %v\n"+
            "WebSocket URL: `%s`\n"+
            "Database Path: `%s`",
        b.config.QueueSize,
        b.config.QuoteInterval,
        b.config.DryRun,
        b.config.WebsocketURL,
        b.config.DatabasePath)
}

func (b *Bot) handleSetConfigCommand(key, value string) (string, error) {
    b.mutex.Lock()
    defer b.mutex.Unlock()

    switch strings.ToLower(key) {
    case "queuesize":
        size, err := strconv.Atoi(value)
        if err != nil {
            return "", fmt.Errorf("invalid queue size: %s", value)
        }
        b.config.QueueSize = size

    case "quoteinterval":
        interval, err := strconv.Atoi(value)
        if err != nil {
            return "", fmt.Errorf("invalid interval: %s", value)
        }
        b.config.QuoteInterval = interval

    case "dryrun":
        dryRun, err := strconv.ParseBool(value)
        if err != nil {
            return "", fmt.Errorf("invalid dryrun value: %s", value)
        }
        b.config.DryRun = dryRun

    default:
        return "", fmt.Errorf("unknown configuration key: %s", key)
    }

    return fmt.Sprintf("Configuration updated: %s = %s", key, value), nil
}

func (b *Bot) handleDryRunCommand(ctx context.Context, mint string, amountStr string) (string, error) {
    amount, err := strconv.ParseFloat(amountStr, 64)
    if err != nil {
        return "", fmt.Errorf("invalid amount: %s", amountStr)
    }

    // Get quote without executing the swap
    quote, err := b.jupiter.GetQuote(ctx, mint)
    if err != nil {
        return "", err
    }

    return fmt.Sprintf(
        "*Dry Run Swap Simulation*\n"+
            "Input: %.4f SOL\n"+
            "Expected Output: %.8f tokens\n"+
            "Price Impact: %.2f%%\n"+
            "Minimum Output: %.8f tokens",
        amount,
        quote.OutAmount,
        quote.PriceImpactPct,
        quote.MinimumOutAmount), nil
}

func (b *Bot) handleSwapCommand(ctx context.Context, mint string, amountStr string) (string, error) {
    if b.config.DryRun {
        return "", fmt.Errorf("bot is in dry run mode. Use /dryrun for simulations")
    }

    amount, err := strconv.ParseFloat(amountStr, 64)
    if err != nil {
        return "", fmt.Errorf("invalid amount: %s", amountStr)
    }

    if err := b.jupiter.ExecuteSwap(ctx, mint, amount, b.config.UserPublicKey); err != nil {
        return "", fmt.Errorf("swap failed: %v", err)
    }

    return fmt.Sprintf("Successfully executed swap of %.4f SOL for token %s", amount, mint), nil
}
