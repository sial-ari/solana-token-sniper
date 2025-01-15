// internal/database/database.go
package database

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"time"
	"github.com/sial-ari/solana-token-sniper/internal/logger"
)

type Database struct {
	db     *sql.DB
	logger *logger.Logger
}

// ... (previous Token and PriceRecord structs remain the same)

func NewDatabase(path string, l *logger.Logger) (*Database, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Create tables if they don't exist
	err = createTables(db)
	if err != nil {
		return nil, err
	}

	return &Database{
		db:     db,
		logger: l,
	}, nil
}

// ... (createTables function remains the same)

func (db *Database) AddToken(token Token) error {
	operation := db.logger.TimeOperation("AddToken")
	defer operation.End()

	_, err := db.db.Exec(`
		INSERT INTO tokens (address, name, symbol, created_at, initial_price)
		VALUES (?, ?, ?, ?, ?)
	`, token.Address, token.Name, token.Symbol, token.CreatedAt, token.InitialPrice)

	if err != nil {
		db.logger.Error(fmt.Sprintf("Failed to add token %s: %v", token.Address, err))
	}

	return err
}

func (db *Database) AddPriceRecord(record PriceRecord) error {
	operation := db.logger.TimeOperation("AddPriceRecord")
	defer operation.End()

	_, err := db.db.Exec(`
		INSERT INTO price_records (token_address, price, timestamp, profit_loss)
		VALUES (?, ?, ?, ?)
	`, record.TokenAddress, record.Price, record.Timestamp, record.ProfitLoss)

	if err != nil {
		db.logger.Error(fmt.Sprintf("Failed to add price record for token %s: %v", record.TokenAddress, err))
	}

	return err
}

func (db *Database) GetTokens() ([]Token, error) {
	operation := db.logger.TimeOperation("GetTokens")
	defer operation.End()

	rows, err := db.db.Query(`SELECT * FROM tokens`)
	if err != nil {
		db.logger.Error(fmt.Sprintf("Failed to query tokens: %v", err))
		return nil, err
	}
	defer rows.Close()

	var tokens []Token
	for rows.Next() {
		var token Token
		err := rows.Scan(&token.Address, &token.Name, &token.Symbol, &token.CreatedAt, &token.InitialPrice)
		if err != nil {
			db.logger.Error(fmt.Sprintf("Failed to scan token row: %v", err))
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}

func (db *Database) GetPriceHistory(tokenAddress string) ([]PriceRecord, error) {
	operation := db.logger.TimeOperation("GetPriceHistory")
	defer operation.End()

	rows, err := db.db.Query(`
		SELECT token_address, price, timestamp, profit_loss
		FROM price_records
		WHERE token_address = ?
		ORDER BY timestamp DESC
	`, tokenAddress)
	if err != nil {
		db.logger.Error(fmt.Sprintf("Failed to query price history for token %s: %v", tokenAddress, err))
		return nil, err
	}
	defer rows.Close()

	var records []PriceRecord
	for rows.Next() {
		var record PriceRecord
		err := rows.Scan(&record.TokenAddress, &record.Price, &record.Timestamp, &record.ProfitLoss)
		if err != nil {
			db.logger.Error(fmt.Sprintf("Failed to scan price record row: %v", err))
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

// Add performance analysis methods
func (db *Database) GetAverageQueryTime(operationType string, timeWindow time.Duration) (time.Duration, error) {
	operation := db.logger.TimeOperation("GetAverageQueryTime")
	defer operation.End()

	var avgTime float64
	err := db.db.QueryRow(`
		SELECT AVG(elapsed_time)
		FROM operation_logs
		WHERE operation_type = ?
		AND timestamp > datetime('now', '-' || ? || ' seconds')
	`, operationType, int(timeWindow.Seconds())).Scan(&avgTime)

	if err != nil {
		return 0, err
	}

	return time.Duration(avgTime) * time.Millisecond, nil
}
