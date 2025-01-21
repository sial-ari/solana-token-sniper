package db

import (
    "database/sql"
    "time"
    _ "github.com/mattn/go-sqlite3"
    "github.com/sial-ari/solana-token-sniper/internal/models"
)

type Database struct {
    db *sql.DB
}

// Initialize creates a new database connection and sets up the schema
func Initialize(dbPath string) (*Database, error) {
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, err
    }

    // Create tables if they don't exist
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS tokens (
            mint TEXT PRIMARY KEY,
            name TEXT,
            symbol TEXT,
            bonding_curve_key TEXT,
            initial_buy REAL,
            market_cap_sol REAL,
            signature TEXT,
            sol_amount REAL,
            trader_public_key TEXT,
            tx_type TEXT,
            uri TEXT,
            v_sol_in_bonding_curve REAL,
            v_tokens_in_bonding_curve REAL,
            created_at DATETIME
        );

        CREATE TABLE IF NOT EXISTS price_history (
            mint TEXT,
            price REAL,
            timestamp DATETIME,
            PRIMARY KEY (mint, timestamp),
            FOREIGN KEY (mint) REFERENCES tokens(mint)
        );

        CREATE TABLE IF NOT EXISTS profit_loss (
            mint TEXT PRIMARY KEY,
            initial_price REAL,
            current_price REAL,
            profit_loss REAL,
            profit_loss_pct REAL,
            last_updated DATETIME,
            FOREIGN KEY (mint) REFERENCES tokens(mint)
        );
    `)

    if err != nil {
        return nil, err
    }

    return &Database{db: db}, nil
}

// SaveNewToken stores a new token in the database
func (d *Database) SaveNewToken(token *models.NewToken) error {
    _, err := d.db.Exec(`
        INSERT INTO tokens (
            mint, name, symbol, bonding_curve_key, initial_buy, 
            market_cap_sol, signature, sol_amount, trader_public_key,
            tx_type, uri, v_sol_in_bonding_curve, v_tokens_in_bonding_curve,
            created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        token.Mint, token.Name, token.Symbol, token.BondingCurveKey,
        token.InitialBuy, token.MarketCapSol, token.Signature,
        token.SolAmount, token.TraderPublicKey, token.TxType,
        token.URI, token.VSolInBondingCurve, token.VTokensInBondingCurve,
        time.Now(),
    )
    return err
}

// SaveTokenPrice stores a new price point for a token
func (d *Database) SaveTokenPrice(price *models.TokenPrice) error {
    _, err := d.db.Exec(`
        INSERT INTO price_history (mint, price, timestamp)
        VALUES (?, ?, ?)`,
        price.Mint, price.Price, price.Timestamp,
    )
    return err
}

// UpdateProfitLoss updates the profit/loss calculation for a token
func (d *Database) UpdateProfitLoss(pl *models.TokenProfitLoss) error {
    _, err := d.db.Exec(`
        INSERT OR REPLACE INTO profit_loss (
            mint, initial_price, current_price, profit_loss,
            profit_loss_pct, last_updated
        ) VALUES (?, ?, ?, ?, ?, ?)`,
        pl.Mint, pl.InitialPrice, pl.CurrentPrice,
        pl.ProfitLoss, pl.ProfitLossPct, pl.LastUpdated,
    )
    return err
}

// GetTokensInQueue retrieves the most recent tokens up to the queue size
func (d *Database) GetTokensInQueue(queueSize int) ([]models.NewToken, error) {
    rows, err := d.db.Query(`
        SELECT mint, name, symbol, bonding_curve_key, initial_buy,
               market_cap_sol, signature, sol_amount, trader_public_key,
               tx_type, uri, v_sol_in_bonding_curve, v_tokens_in_bonding_curve,
               created_at
        FROM tokens
        ORDER BY created_at DESC
        LIMIT ?`,
        queueSize,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var tokens []models.NewToken
    for rows.Next() {
        var t models.NewToken
        err := rows.Scan(
            &t.Mint, &t.Name, &t.Symbol, &t.BondingCurveKey,
            &t.InitialBuy, &t.MarketCapSol, &t.Signature,
            &t.SolAmount, &t.TraderPublicKey, &t.TxType,
            &t.URI, &t.VSolInBondingCurve, &t.VTokensInBondingCurve,
            &t.CreatedAt,
        )
        if err != nil {
            return nil, err
        }
        tokens = append(tokens, t)
    }
    return tokens, nil
}

// GetPriceHistory retrieves the price history for a specific token
func (d *Database) GetPriceHistory(mint string) ([]models.TokenPrice, error) {
    rows, err := d.db.Query(`
        SELECT price, timestamp
        FROM price_history
        WHERE mint = ?
        ORDER BY timestamp DESC`,
        mint,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var prices []models.TokenPrice
    for rows.Next() {
        var p models.TokenPrice
        p.Mint = mint
        err := rows.Scan(&p.Price, &p.Timestamp)
        if err != nil {
            return nil, err
        }
        prices = append(prices, p)
    }
    return prices, nil
}
