package models

import "time"

type NewToken struct {
    BondingCurveKey       string    `json:"bondingCurveKey"`
    InitialBuy            float64   `json:"initialBuy"`
    MarketCapSol          float64   `json:"marketCapSol"`
    Mint                  string    `json:"mint"`
    Name                  string    `json:"name"`
    Signature             string    `json:"signature"`
    SolAmount             float64   `json:"solAmount"`
    Symbol                string    `json:"symbol"`
    TraderPublicKey       string    `json:"traderPublicKey"`
    TxType                string    `json:"txType"`
    URI                   string    `json:"uri"`
    VSolInBondingCurve    float64   `json:"vSolInBondingCurve"`
    VTokensInBondingCurve float64   `json:"vTokensInBondingCurve"`
    CreatedAt             time.Time `json:"createdAt"`
}

type TokenPrice struct {
    Mint      string    `json:"mint"`
    Price     float64   `json:"price"`
    Timestamp time.Time `json:"timestamp"`
}

type TokenProfitLoss struct {
    Mint           string    `json:"mint"`
    InitialPrice   float64   `json:"initialPrice"`
    CurrentPrice   float64   `json:"currentPrice"`
    ProfitLoss     float64   `json:"profitLoss"`
    ProfitLossPct  float64   `json:"profitLossPct"`
    LastUpdated    time.Time `json:"lastUpdated"`
}
