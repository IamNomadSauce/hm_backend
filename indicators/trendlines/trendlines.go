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
		current = common.Trendline{
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
				current = common.Trendline{
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
				current = common.Trendline{
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
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`, tableName))
		if err != nil {
			return fmt.Errorf("error creating trendlines table %s: %w", tableName, err)
		}
		log.Printf("Created table %s", tableName)
	}

	log.Println("Generate trendlines for all of the candles")
	current, err := t.getCurrentTrendline(t.asset, t.timeframe, t.exchange)
	if err != nil {
		return fmt.Errorf("error fetching current trendline: %w", err)
	}

	log.Println("current", current)

	// if current.StartTime == 0 {
	// 	candles, err := t.fetchHistoricalCandles(t.asset, t.timeframe, t.exchange)
	// 	if err != nil {
	// 		return fmt.Errorf("error fetching historical candles: %w", err)
	// 	}
	// 	log.Printf("Processing %d historical candles for %s %s %s", len(candles), t.asset, t.timeframe, t.exchange)
	// 	for _, candle := range candles {
	// 		if err := t.ProcessCandle(t.asset, t.timeframe, t.exchange, candle); err != nil {
	// 			log.Printf("Error processing historical candle: %v", err)
	// 		}
	// 	}
	// } else {
	// 	log.Printf("Current trendline exists for %s %s %s, skipping historical processing", t.asset, t.timeframe, t.exchange)
	// }

	return nil
}

func (t *TrendlineIndicator) processTrendlines(asset, timeframe, exchange string, candles []common.Candle) ([]common.Trendline, error) {

	// If there are no trendlines, start from scratch
	// If there are trendlines, use the current trend from the table

	var current common.Trendline
	var trendlines []common.Trendline
	var counter int

	for index, candle := range candles {
		if current.Direction == "up" {

			// Lower Low in uptrend (new trend)
			if candle.Low < current.EndInv || (index > 0 && candle.Low < candles[index-1].Low) {

				counter++

				if counter >= 1 {
					// fmt.Println(current)
					current.Done = "done"
					// fmt.Println(current)
					// fmt.Println(len(trendlines))
					if current.StartTime == candle.Timestamp {
						fmt.Println("DUPLICATE!", current.StartTime)
					}
					// if candle.High > current.End {
					// 	current.End = candle.High
					// 	current.EndTime = candle.Time
					// 	current.EndInv = candle.Low
					// 	current.EndTS = candle.Open
					// }
					trendlines = append(trendlines, current)
					temp := current

					current = common.Trendline{
						StartPrice: temp.EndPrice,
						StartTime:  temp.EndTime,
						StartInv:   temp.EndInv,
						// StartTS:   temp.EndTS,
						EndPrice: candle.Low,
						EndTime:  candle.Timestamp,
						EndInv:   candle.High,
						// EndTS:     candle.Close,
						Direction: "down",
						Done:      "current",
					}
					// fmt.Println(current)
					// fmt.Println("Time:", current.Time)

					counter = 0
				}
				continue

			}

			// Higher High in uptrend  (continuation)
			if candle.High > current.EndPrice {
				current = common.Trendline{
					StartTime:  current.StartTime,
					StartPrice: current.StartPrice,
					StartInv:   current.StartInv,
					// StartTS:   current.StartTS,
					EndTime:  candle.Timestamp,
					EndPrice: candle.High,
					EndInv:   candle.Low,
					// EndTS:     candle.Open,
					Direction: "up",
					Done:      "current",
				}
				// fmt.Println("Time:", current.Time)

				counter = 0
				continue
			}

		} else if current.Direction == "down" {

			// Higher High in downtrend  (new trend)
			if candle.High > current.EndInv || (index > 0 && candle.High > candles[index-1].High) {
				counter++

				if counter >= 1 {
					current.Done = "done"

					if current.StartTime == candle.Timestamp {
						fmt.Println("DUPLICATE!", current.StartTime)
					}
					// if candle.High > current.EndInv {

					// }
					// if candle.Low < current.End {
					// 	current.End = candle.Low
					// 	current.EndTime = candle.Time
					// 	current.EndInv = candle.High
					// 	current.EndTS = candle.Close
					// }

					trendlines = append(trendlines, current)
					temp := current

					current = common.Trendline{
						StartPrice: temp.EndPrice,
						StartTime:  temp.EndTime,
						StartInv:   temp.EndInv,
						// StartTS:   temp.EndTS,
						EndPrice: candle.High,
						EndTime:  candle.Timestamp,
						EndInv:   candle.Low,
						// EndTS:     candle.Open,
						Direction: "up",
						Done:      "current",
					}
					counter = 0
				}

				continue

			}

			// Lower Low in downtrend  (continuation)
			if candle.Low < current.EndPrice {
				current = common.Trendline{
					StartPrice: current.StartPrice,
					StartTime:  current.StartTime,
					StartInv:   current.StartInv,
					// StartTS:   current.StartTS,
					EndPrice: candle.Low,
					EndTime:  candle.Timestamp,
					EndInv:   candle.High,
					// EndTS:     candle.Close,
					Direction: "down",
					Done:      "current",
				}
				// fmt.Println("Time:", current.Time)

				counter = 0

				continue

			}

		}
		// if current.StartTime == current.EndTime {
		// 	current.StartTime = current.StartTime - 1
		// }
	}
	return trendlines, nil
}

// MakeTrendlines generates trendlines based on the given candles.
func MakeTrendlines(candles []common.Candle) []common.Trendline {
	var trendlines []common.Trendline

	// Return empty slice if no candles are provided
	if len(candles) == 0 {
		return trendlines
	}

	// Initialize start and end points from the first candle
	start := common.Point{
		Time:       candles[0].Timestamp,
		Point:      candles[0].Low,
		TrendStart: candles[0].Close,
		Inv:        candles[0].Close,
	}
	end := common.Point{
		Time:       candles[0].Timestamp,
		Point:      candles[0].High,
		TrendStart: candles[0].Close,
		Inv:        candles[0].Close,
	}
	current := common.Trendline{
		Start:  start,
		End:    end,
		Type:   "up",
		Status: "current",
	}

	// Process each candle
	for i, candle := range candles {
		if current.Type == "up" {
			// Higher High in uptrend (continuation)
			if candle.High > current.End.Point {
				current.End = common.Point{
					Time:       candle.Timestamp,
					Point:      candle.High,
					Inv:        candle.Low,
					TrendStart: max(candle.Close, candle.Open),
				}
			}
			// Lower Low in uptrend (new trend)
			if (candle.Low < current.End.Inv) || (i > 0 && candle.Low < candles[i-1].Low) {
				current.Status = "done"
				trendlines = append(trendlines, current)
				current = common.Trendline{
					Start: current.End,
					End: common.Point{
						Time:       candle.Timestamp,
						Point:      candle.Low,
						Inv:        candle.High,
						TrendStart: min(candle.Close, candle.Open),
					},
					Type:   "down",
					Status: "current",
				}
			}
		} else if current.Type == "down" {
			// Lower Low in downtrend (continuation)
			if candle.Low < current.End.Point {
				current.End = common.Point{
					Time:       candle.Timestamp,
					Point:      candle.Low,
					Inv:        candle.High,
					TrendStart: min(candle.Close, candle.Open),
				}
			}
			// Higher High in downtrend (new trend)
			if (candle.High > current.End.Inv) || (i > 0 && candle.High > candles[i-1].High) {
				current.Status = "done"
				trendlines = append(trendlines, current)
				current = common.Trendline{
					Start: current.End,
					End: common.Point{
						Time:       candle.Timestamp,
						Point:      candle.High,
						Inv:        candle.Low,
						TrendStart: max(candle.Close, candle.Open),
					},
					Type:   "up",
					Status: "current",
				}
			}
		}
	}

	// Append the last current trendline
	trendlines = append(trendlines, current)
	return trendlines
}

// max returns the maximum of two float64 values
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
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

func (t *TrendlineIndicator) getCurrentTrendline(asset, timeframe, exchange string) (common.Trendline, error) {
	tableName := fmt.Sprintf("trendlines_%s_%s_%s", strings.ReplaceAll(strings.ToLower(asset), "-", "_"), timeframe, exchange)
	var tr common.Trendline
	query := fmt.Sprintf("SELECT id, start_time, start_price, end_time, end_price, direction, done FROM %s WHERE done = 'current' LIMIT 1", tableName)
	err := t.db.QueryRow(query).Scan(&tr.ID, &tr.StartTime, &tr.StartPrice, &tr.EndTime, &tr.EndPrice, &tr.Direction, &tr.Done)
	if err == sql.ErrNoRows {
		return common.Trendline{}, nil
	}
	return tr, err
}

func (t *TrendlineIndicator) insertTrendline(asset, timeframe, exchange string, tr common.Trendline) error {
	tableName := fmt.Sprintf("trendlines_%s_%s_%s", strings.ReplaceAll(strings.ToLower(asset), "-", "_"), timeframe, exchange)
	query := fmt.Sprintf(`
		INSERT INTO %s(start_time, start_price, end_time, end_price, direction, done)
		VALUES ($1, $2, $3, $4, $5, $6)`, tableName)
	_, err := t.db.Exec(query, tr.StartTime, tr.StartPrice, tr.EndTime, tr.EndPrice, tr.Direction, tr.Done)
	return err
}

func (t *TrendlineIndicator) updateCurrentTrendline(asset, timeframe, exchange string, tr common.Trendline) error {
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
