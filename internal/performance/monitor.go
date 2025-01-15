// internal/performance/monitor.go
package performance

import (
    "sync"
    "time"
    "github.com/sial-ari/solana-token-sniper/internal/logger"
)

type OperationType string

const (
    OpTokenCreate    OperationType = "token_create"
    OpPriceCheck     OperationType = "price_check"
    OpJupiterSwap    OperationType = "jupiter_swap"
    OpDatabaseWrite  OperationType = "db_write"
    OpDatabaseRead   OperationType = "db_read"
)

type Metric struct {
    OperationType OperationType
    Duration     time.Duration
    Timestamp    time.Time
    Success      bool
}

type Monitor struct {
    metrics []Metric
    mu      sync.RWMutex
    logger  *logger.Logger
}

func NewMonitor(l *logger.Logger) *Monitor {
    return &Monitor{
        metrics: make([]Metric, 0),
        logger:  l,
    }
}

func (m *Monitor) RecordMetric(opType OperationType, duration time.Duration, success bool) {
    m.mu.Lock()
    defer m.mu.Unlock()

    metric := Metric{
        OperationType: opType,
        Duration:     duration,
        Timestamp:    time.Now(),
        Success:      success,
    }

    m.metrics = append(m.metrics, metric)
    m.logger.Info(fmt.Sprintf("Operation %s completed in %v (success: %v)", opType, duration, success))
}

func (m *Monitor) GetAverageLatency(opType OperationType, window time.Duration) time.Duration {
    m.mu.RLock()
    defer m.mu.RUnlock()

    var total time.Duration
    var count int

    cutoff := time.Now().Add(-window)
    for _, metric := range m.metrics {
        if metric.OperationType == opType && metric.Timestamp.After(cutoff) && metric.Success {
            total += metric.Duration
            count++
        }
    }

    if count == 0 {
        return 0
    }

    return total / time.Duration(count)
}

func (m *Monitor) GetSuccessRate(opType OperationType, window time.Duration) float64 {
    m.mu.RLock()
    defer m.mu.RUnlock()

    var total, successful int
    cutoff := time.Now().Add(-window)

    for _, metric := range m.metrics {
        if metric.OperationType == opType && metric.Timestamp.After(cutoff) {
            total++
            if metric.Success {
                successful++
            }
        }
    }

    if total == 0 {
        return 0
    }

    return float64(successful) / float64(total) * 100
}

func (m *Monitor) PruneOldMetrics(retention time.Duration) {
    m.mu.Lock()
    defer m.mu.Unlock()

    cutoff := time.Now().Add(-retention)
    newMetrics := make([]Metric, 0)

    for _, metric := range m.metrics {
        if metric.Timestamp.After(cutoff) {
            newMetrics = append(newMetrics, metric)
        }
    }

    m.metrics = newMetrics
}
