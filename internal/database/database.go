// internal/database/database.go
package database

import (
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
    "time"
)

type Database struct {
    db *sql.DB
}

type Token struct {
    Address     string    `json:"address"`
    Name        string    `json:"name"`
    Symbol      string    `json:"symbol"`
    CreatedAt   time.Time `json:"created_at"`
    InitialPrice float64   `json:"initial_price"`
}

type PriceRecord struct {
    TokenAddress string    `json:"token_address"`
    Price       float64    `json:"price"`
    Timestamp   time.Time  `json:"timestamp"`
    ProfitLoss  float64    `json:"profit_loss"`
}

func NewDatabase(path string) (*Database, error) {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return nil, err
    }

    // Create tables if they don't exist
    err = createTables(db)
    if err != nil {
        return nil, err
    }

    return &Database{db: db}, nil
}

func createTables(db *sql.DB) error {
    // Create tokens table
    _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS tokens (
            address TEXT PRIMARY KEY,
            name TEXT,
            symbol TEXT,
            created_at DATETIME,
            initial_price REAL
        )
    `)
    if err != nil {
        return err
    }

    // Create price_records table
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS price_records (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            token_address TEXT,
            price REAL,
            timestamp DATETIME,
            profit_loss REAL,
            FOREIGN KEY(token_address) REFERENCES tokens(address)
        )
    `)
    if err != nil {
        return err
    }

    return nil
}

// Add methods for database operations
func (db *Database) AddToken(token Token) error {
    _, err := db.db.Exec(`
        INSERT INTO tokens (address, name, symbol, created_at, initial_price)
        VALUES (?, ?, ?, ?, ?)
    `, token.Address, token.Name, token.Symbol, token.CreatedAt, token.InitialPrice)
    return err
}

func (db *Database) AddPriceRecord(record PriceRecord) error {
    _, err := db.db.Exec(`
        INSERT INTO price_records (token_address, price, timestamp, profit_loss)
        VALUES (?, ?, ?, ?)
    `, record.TokenAddress, record.Price, record.Timestamp, record.ProfitLoss)
    return err
}

func (db *Database) GetTokens() ([]Token, error) {
    rows, err := db.db.Query(`SELECT * FROM tokens`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var tokens []Token
    for rows.Next() {
        var token Token
        err := rows.Scan(&token.Address, &token.Name, &token.Symbol, &token.CreatedAt, &token.InitialPrice)
        if err != nil {
            return nil, err
        }
        tokens = append(tokens, token)
    }
    return tokens, nil
}

func (db *Database) GetPriceHistory(tokenAddress string) ([]PriceRecord, error) {
    rows, err := db.db.Query(`
        SELECT token_address, price, timestamp, profit_loss
        FROM price_records
        WHERE token_address = ?
        ORDER BY timestamp DESC
    `, tokenAddress)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var records []PriceRecord
    for rows.Next() {
        var record PriceRecord
        err := rows.Scan(&record.TokenAddress, &record.Price, &record.Timestamp, &record.ProfitLoss)
        if err != nil {
            return nil, err
        }
        records = append(records, record)
    }
    return records, nil
}
