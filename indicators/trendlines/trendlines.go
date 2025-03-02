package trendlines

import (
	"backend/common"
	"backend/sse"
	"database/sql"
	"encoding/json"
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
	asset      string
	timeframe  string
	exchange   string
}

func NewTrendlineIndicator(db *sql.DB, sseManager *sse.SSEManager, asset, timeframe, exchange string) *TrendlineIndicator {
	return &TrendlineIndicator{
		db:         db,
		sseManager: sseManager,
		asset:      asset,
		timeframe:  timeframe,
		exchange:   exchange,
	}
}

func (t *TrendlineIndicator) ProcessCandle(asset, timeframe, exchange string, candle common.Candle) error {
	log.Printf("Processing trendline Candle for %s %s: Time=%v, High=%f, Low=%f", asset, timeframe, candle.Timestamp, candle.High, candle.Low)

	current, err := t.getCurrentTrendline(asset, timeframe, exchange)
	if err != nil {
		return fmt.Errorf("error fetching current trendline: %w", err)
	}
	if current.StartTime == 0 {
		current = Trendline{
			StartTime:  candle.Timestamp,
			StartPrice: candle.High,
			EndTime:    candle.Timestamp,
			EndPrice:   candle.High,
			Direction:  "up",
			Done:       "current",
		}
	} else {
		if current.Direction == "up" {
			if candle.High > current.EndPrice {
				current.EndTime = candle.Timestamp
				current.EndPrice = candle.High
			} else if candle.Low < current.EndPrice {
				current.Done = "done"
				if err := t.insertTrendline(asset, timeframe, exchange, current); err != nil {
					log.Printf("Error inserting trendline: %v", err)
				}
				current = Trendline{
					StartTime:  candle.Timestamp,
					StartPrice: candle.Low,
					EndTime:    candle.Timestamp,
					EndPrice:   candle.Low,
					Direction:  "down",
					Done:       "current",
				}
			}
		} else {
			if candle.Low < current.EndPrice {
				current.EndTime = candle.Timestamp
				current.EndPrice = candle.Low
			} else if candle.High > current.EndPrice {
				current.Done = "done"
				if err := t.insertTrendline(asset, timeframe, exchange, current); err != nil {
					log.Printf("Error inserting trendline: %v", err)
				}
				current = Trendline{
					StartTime:  candle.Timestamp,
					StartPrice: candle.High,
					EndTime:    candle.Timestamp,
					EndPrice:   candle.High,
					Direction:  "up",
					Done:       "current",
				}
			}
		}
	}

	if err := t.updateCurrentTrendline(asset, timeframe, exchange, current); err != nil {
		return fmt.Errorf("error updating current trendline: %w", err)
	}

	message := struct {
		Event string        `json:"event"`
		Data  common.Candle `json:"data"`
	}{
		Event: "trendline",
		Data:  candle,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal candle message to JSON: %w", err)
	}

	t.sseManager.BroadcastMessage(string(jsonData))

	return nil
}

func (t *TrendlineIndicator) ParseTrendlines(candles []common.Candle) {

}

func (t *TrendlineIndicator) Start() error {
	log.Println("Trendline Indicator Start")

	tableName := fmt.Sprintf("trendlines_%s_%s_%s", strings.ReplaceAll(strings.ToLower(t.asset), "-", "_"), t.timeframe, t.exchange)

	var exists bool
	err := t.db.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", tableName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error checking if table exists: %w", err)
	}

	if !exists {
		_, err = t.db.Exec(fmt.Sprintf(`
			CREATE TABLE %s (
				id SERIAL PRIMARY KEY,
				start_time BIGINT,
				start_price FLOAT,
				end_time BIGINT,
				end_price FLOAT,
				direction VARCHAR(10),
				done VARCHAR(10),
				dupdated_ap TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`, tableName))
		if err != nil {
			return fmt.Errorf("error creating trendlines table %s: %w", tableName, err)
		}
		log.Printf("Created table %s", tableName)
	}

	current, err := t.getCurrentTrendline(t.asset, t.timeframe, t.exchange)
	if err != nil {
		return fmt.Errorf("error fetching current trendline: %w", err)
	}

	if current.StartTime == 0 {
		candles, err := t.fetchHistoricalCandles(t.asset, t.timeframe, t.exchange)
		if err != nil {
			return fmt.Errorf("error fetching historical candles: %w", err)
		}
		log.Printf("Processing %d historical candles for %s %s %s", len(candles), t.asset, t.timeframe, t.exchange)
		for _, candle := range candles {
			if err := t.ProcessCandle(t.asset, t.timeframe, t.exchange, candle); err != nil {
				log.Printf("Error processing historical candle: %v", err)
			}
		}
	} else {
		log.Printf("Current trendline exists for %s %s %s, skipping historical processing", t.asset, t.timeframe, t.exchange)
	}

	return nil
}

func (t *TrendlineIndicator) fetchHistoricalCandles(asset, timeframe, exchange string) ([]common.Candle, error) {
	tableName := fmt.Sprintf("%s_%s_%s", strings.ReplaceAll(strings.ToLower(asset), "-", "_"), timeframe, exchange)
	query := fmt.Sprintf("SELECT timestamp, open, high, low, close, volume FROM %s ORDER BY timestamp ASC", tableName)
	rows, err := t.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying candles from %s: %w", tableName, err)
	}
	defer rows.Close()

	var candles []common.Candle
	for rows.Next() {
		var c common.Candle
		if err := rows.Scan(&c.Timestamp, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume); err != nil {
			return nil, fmt.Errorf("error scanning candle: %w", err)
		}
		c.ProductID = strings.ToUpper(asset)
		candles = append(candles, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating candle rows: %w", err)
	}

	return candles, nil
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
	tableName := fmt.Sprintf("trendlines_%s_%s_%s", strings.ReplaceAll(strings.ToLower(asset), "-", "_"), timeframe, exchange)
	query := fmt.Sprintf(`
		INSERT INTO %s(start_time, start_price, end_time, end_price, direction, done)
		VALUES ($1, $2, $3, $4, $5, $6)`, tableName)
	_, err := t.db.Exec(query, tr.StartTime, tr.StartPrice, tr.EndTime, tr.EndPrice, tr.Direction, tr.Done)
	return err
}

func (t *TrendlineIndicator) updateCurrentTrendline(asset, timeframe, exchange string, tr Trendline) error {
	tableName := fmt.Sprintf("trendlines_%s_%s_%s", strings.ReplaceAll(strings.ToLower(asset), "-", "_"), timeframe, exchange)
	var id int
	query := fmt.Sprintf("SELECT id FROM %s WHERE done = 'current' LIMIT 1", tableName)
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
