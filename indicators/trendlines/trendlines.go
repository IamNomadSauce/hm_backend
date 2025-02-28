package trendlines

import (
	"backend/common"
	"backend/sse"
	"database/sql"
	"fmt"
	"log"
	"strings"
)

type Trendline struct {
	ID         int
	StartTime  int64
	StartPrice float64
	EndTime    int64
	EndPrice   float64
	Direction  string
	Done       string
}

type TrendlineIndicator struct {
	db         *sql.DB
	sseManager *sse.SSEManager
}

func NewTrendlineIndicator(db *sql.DB, sseManager *sse.SSEManager) *TrendlineIndicator {
	return &TrendlineIndicator{
		db:         db,
		sseManager: sseManager,
	}
}

func (t *TrendlineIndicator) ProcessCandle(asset, timeframe string, candle common.Candle) error {
	log.Printf("Processing trendline Candle for %s %s: Time=%v, High=%f, Low=%f", asset, timeframe, candle.Timestamp, candle.High, candle.Low)

	t.sseManager.BroadcastMessage(fmt.Sprintf("Processed candle for %s %s at %d", asset, timeframe, candle.Timestamp))
	// current, err := t.getCurrentTrendline(asset, timeframe, exchange)
	// if err != nil {
	// 	return fmt.Errorf("error fetching current trendline: %w", err)
	// }
	// if current.StartTime.IsZero() {
	// 	current = Trendline{
	// 		StartTime:  candle.Timestamp,
	// 		StartPrice: candle.High,
	// 		EndTime:    candle.Timestamp,
	// 		EndPrice:   candle.High,
	// 		Direction:  "up",
	// 		Done:       "current",
	// 	}
	// }

	// if candle.High > current.EndPrice && current.Direction == "up" {
	// 	current.EndTime = candle.Timestamp
	// 	current.EndPrice = candle.High
	// } else if candle.Low < current.EndPrice && current.Direction == "up" {
	// 	current.Done = "done"
	// 	if err := t.insertTrendline(asset, timeframe, exchange, current); err != nil {
	// 		log.Printf("Error inserting trendline: %v", err)
	// 	}
	// 	current = Trendline{
	// 		StartTime:  candle.Timestamp,
	// 		StartPrice: candle.Low,
	// 		EndTime:    candle.Timestamp,
	// 		EndPrice:   candle.Low,
	// 		Direction:  "down",
	// 		Done:       "current",
	// 	}
	// }

	// if err := t.updateCurrentTrendline(asset, timeframe, exchange, current); err != nil {
	// 	return fmt.Errorf("error updating current trendline: %w", err)
	// }

	return nil
}

func (t *TrendlineIndicator) Start() error {
	return nil
}

func (t *TrendlineIndicator) getCurrentTrendline(asset, timeframe, exchange string) (Trendline, error) {
	tableName := fmt.Sprintf("trendlines_%s_%s_%s", strings.ReplaceAll(strings.ToLower(asset), "-", "_"), timeframe, exchange)
	var tr Trendline
	query := fmt.Sprintf("SELECT id, start_time, start_price, end_time, end_price, direction, done FROM %s WHERE done = 'current' LIMIT 1", tableName)
	err := t.db.QueryRow(query).Scan(&tr.ID, &tr.StartTime, &tr.StartPrice, &tr.EndTime, &tr.EndPrice, &tr.Direction, &tr.Done)
	if err == sql.ErrNoRows {
		return Trendline{}, nil
	}
	return tr, err
}

func (t *TrendlineIndicator) insertTrendline(asset, timeframe, exchange string, tr Trendline) error {
	tableName := fmt.Sprintf("trendlines_%s_%s_%s", strings.ReplaceAll(strings.ToLower(asset), "-", "_"), timeframe)
	query := fmt.Sprintf(`
		INSERT INTO %s(start_time, start_price, end_time, end_price, direction, done)
		VALUES ($1, $2, $3, $4, $5, $6)`, tableName)
	_, err := t.db.Exec(query, tr.StartTime, tr.StartPrice, tr.EndTime, tr.EndPrice, tr.Direction, tr.Done)
	return err
}

func (t *TrendlineIndicator) updateCurrentTrendline(asset, timeframe, exchange string, tr Trendline) error {
	tableName := fmt.Sprintf("trendlines_%s_%s_%s", strings.ReplaceAll(strings.ToLower(asset), "-", "_"), timeframe, exchange)
	var id int
	query := fmt.Sprintf("SELECT id FROM %S WHERE done = 'current' LIMIT 1", tableName)
	err := t.db.QueryRow(query).Scan(&id)
	if err == sql.ErrNoRows {
		return t.insertTrendline(asset, timeframe, exchange, tr)
	} else if err != nil {
		return err
	}

	updateQuery := fmt.Sprintf(`
		UPDATE %s SET 
			start_time = $1, start_price = $2,
			end_time = $3, end_price = $4,
			direction = $5, done = $6,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $7`, tableName)
	_, err = t.db.Exec(updateQuery, tr.StartTime, tr.StartPrice, tr.EndTime, tr.EndPrice, tr.Direction, tr.Done, id)

	return err
}
