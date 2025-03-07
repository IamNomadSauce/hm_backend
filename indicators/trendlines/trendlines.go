package trendlines

import (
	"backend/common"
	"backend/sse"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/lib/pq"
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

	// current, err := t.getCurrentTrendline(asset, timeframe, exchange)
	// if err != nil {
	// 	return fmt.Errorf("error fetching current trendline: %w", err)
	// }
	// if current.StartTime == 0 {
	// 	current = common.Trendline{
	// 		StartTime:  candle.Timestamp,
	// 		StartPrice: candle.High,
	// 		EndTime:    candle.Timestamp,
	// 		EndPrice:   candle.High,
	// 		Direction:  "up",
	// 		Done:       "current",
	// 	}
	// } else {
	// 	if current.Type == "up" {
	// 		if candle.High > current.End.Point {
	// 			current.End.Time = candle.Timestamp
	// 			current.End.Point = candle.High
	// 		} else if candle.Low < current.End.Point {
	// 			current.Type = "done"
	// 			if err := t.insertTrendline(asset, timeframe, exchange, current); err != nil {
	// 				log.Printf("Error inserting trendline: %v", err)
	// 			}
	// 			current = common.Trendline{
	// 				StartTime:  candle.Timestamp,
	// 				StartPrice: candle.Low,
	// 				EndTime:    candle.Timestamp,
	// 				EndPrice:   candle.Low,
	// 				Direction:  "down",
	// 				Done:       "current",
	// 			}
	// 		}
	// 	} else {
	// 		if candle.Low < current.End.Point {
	// 			current.End.Time = candle.Timestamp
	// 			current.End.Point = candle.Low
	// 		} else if candle.High > current.End.Point {
	// 			current.Status = "done"
	// 			if err := t.insertTrendline(asset, timeframe, exchange, current); err != nil {
	// 				log.Printf("Error inserting trendline: %v", err)
	// 			}
	// 			current = common.Trendline{
	// 				StartTime:  candle.Timestamp,
	// 				StartPrice: candle.High,
	// 				EndTime:    candle.Timestamp,
	// 				EndPrice:   candle.High,
	// 				Direction:  "up",
	// 				Done:       "current",
	// 			}
	// 		}
	// }
	// }

	// if err := t.updateCurrentTrendline(asset, timeframe, exchange, current); err != nil {
	// 	return fmt.Errorf("error updating current trendline: %w", err)
	// }

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
				start_time BIGINT,
				start_point FLOAT,
				start_inv FLOAT,
				start_trendstart FLOAT,
				end_time BIGINT,
				end_point FLOAT,
				end_inv FLOAT,
				end_trendstart FLOAT,
				direction VARCHAR(10),
				status VARCHAR(10)
			)`, tableName))
		if err != nil {
			return fmt.Errorf("error creating trendlines table %s: %w", tableName, err)
		}
		log.Printf("Created table %s", tableName)
		candles, err := t.fetchHistoricalCandles(t.asset, t.timeframe, t.exchange)
		if err != nil {
			log.Printf("Error fetching historical candles for %s: %v", tableName, err)
			return fmt.Errorf("error fetching historical candles %v", err)
		}
		trendlines, err := MakeTrendlines(candles)
		if err != nil {
			log.Printf("Error making trendlines for %s: %v", tableName, err)
			return fmt.Errorf("error making trendlines for %s: %v", tableName, err)
		}
		log.Println("Trendlines:", tableName, len(trendlines))
	}

	log.Println("TABLE NAME", tableName)
	stmt := fmt.Sprintf("SELECT COUNT(*) FROM %s", pq.QuoteIdentifier(tableName))
	var count int
	err = t.db.QueryRow(stmt).Scan(&count)
	if err != nil {
		log.Printf("Error getting count for trendline table %s: %v", tableName, err)
	}

	log.Println("Trendline Count", count)
	if count == 0 {
		log.Printf("trendlines empty for %s", tableName)
		log.Println("Generate trendlines for all of the candles")
		candles, err := t.fetchHistoricalCandles(t.asset, t.timeframe, t.exchange)
		if err != nil {
			log.Printf("Error getting all candles for %s: %v", tableName, err)
			return fmt.Errorf("error getting all candles for %s: %v", tableName, err)
		}
		log.Println("Candles:", len(candles))
		trendlines, err := MakeTrendlines(candles)
		if err != nil {
			log.Printf("error making trendlines for %s: %v", tableName, err)
			return fmt.Errorf("error making trendlines for %s: %v", tableName, err)
		}
		log.Println("Trendlines:", len(trendlines))
		for _, trend := range trendlines {
			t.insertTrendline(t.asset, t.timeframe, t.exchange, trend)
		}
		log.Printf("wrote trends to db table for %s", tableName)

	}

	current, err := t.getCurrentTrendline(t.asset, t.timeframe, t.exchange)
	if err == sql.ErrNoRows {
	} else if err != nil {
		return fmt.Errorf("error fetching current trendline: %w", err)
	}

	log.Printf("current %+v", current.Start.Time)

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

// MakeTrendlines generates trendlines based on the given candles.
func MakeTrendlines(candles []common.Candle) ([]common.Trendline, error) {
	var trendlines []common.Trendline

	// Return empty slice if no candles are provided
	if len(candles) == 0 {
		return trendlines, fmt.Errorf("No candles given to makeTrendlines")
	}

	counter := 0
	// startT := time.Now()

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
		Start:     start,
		End:       end,
		Direction: "up",
		Status:    "current",
	}

	for i, candle := range candles {
		// Condition 1: Higher high in uptrend (continuation)
		if candle.High > current.End.Point && current.Direction == "up" {
			current.End = common.Point{
				Time:       candle.Timestamp,
				Point:      candle.High,
				Inv:        candle.Low,
				TrendStart: math.Max(candle.Close, candle.Open), // (close > open) ? close : open
			}
			counter = 0
		} else if (candle.High > current.End.Inv || (i > 0 && candle.High > candles[i-1].High)) && current.Direction == "down" { // Condition 2: Higher high in downtrend (new uptrend)
			counter++
			if counter >= 3 { // Confirm reversal after 3 higher highs
				current.Status = "done"
				trendlines = append(trendlines, current)
				current = common.Trendline{
					Start: current.End,
					End: common.Point{
						Time:       candle.Timestamp,
						Point:      candle.High,
						Inv:        candle.Low,
						TrendStart: math.Max(candle.Close, candle.Open),
					},
					Direction: "up",
					Status:    "current",
				}
				counter = 0
			}
		} else if (candle.Low < current.End.Inv || (i > 0 && candle.Low < candles[i-1].Low)) && current.Direction == "up" { // Condition 3: Lower low in uptrend (new downtrend)
			counter++
			if counter >= 3 { // Confirm reversal after 3 lower lows
				current.Status = "done"
				trendlines = append(trendlines, current)
				current = common.Trendline{
					Start: current.End,
					End: common.Point{
						Time:       candle.Timestamp,
						Point:      candle.Low,
						Inv:        candle.High,
						TrendStart: math.Min(candle.Close, candle.Open), // (close > open) ? open : close
					},
					Direction: "down",
					Status:    "current",
				}
				counter = 0
			}
		} else if candle.Low < current.End.Point && current.Direction == "down" { // Condition 4: Lower low in downtrend (continuation)
			current.End = common.Point{
				Time:       candle.Timestamp,
				Point:      candle.Low,
				Inv:        candle.High,
				TrendStart: math.Min(candle.Close, candle.Open),
			}
			counter = 0
		}
	}

	// stopT := time.Since(startT)
	// log.Printf("Trendlines Time: %.3f seconds\n", stopT.Seconds())
	// log.Println("Trendlines:", trendlines)

	return trendlines, nil
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
	log.Println("FetchHistoricalCandles")
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
	var startTime, endTime int64
	var startPoint, startInv, startTrendStart, endPoint, endInv, endTrendStart float64
	var direction, status string

	query := fmt.Sprintf(`
		SELECT start_time, start_point, start_inv, start_trendstart, end_time, end_point, end_inv, end_trendstart, direction, status
		FROM %s 
		WHERE status = 'current' 
		LIMIT 1
	`, tableName)
	err := t.db.QueryRow(query).Scan(&startTime, &startPoint, &startInv, &startTrendStart, &endTime, &endPoint, &endInv, &endTrendStart, &direction, &status)
	if err == sql.ErrNoRows {
		log.Println("No results found for trendlines")
		return common.Trendline{}, fmt.Errorf(err.Error())
	} else if err != nil {
		log.Println("Error doing trendlines %v", err)
		return common.Trendline{}, fmt.Errorf("Error getting current trendline: %v", err)
	}

	tr = common.Trendline{
		Start: common.Point{
			Time:       startTime,
			Point:      startPoint,
			Inv:        startInv,
			TrendStart: startTrendStart,
		},
		End: common.Point{
			Time:       endTime,
			Point:      endPoint,
			Inv:        endInv,
			TrendStart: endTrendStart,
		},
		Direction: direction,
		Status:    status,
	}

	return tr, err
}

func (t *TrendlineIndicator) insertTrendline(asset, timeframe, exchange string, tr common.Trendline) error {
	// log.Println("insertTrendlines:", asset, timeframe, exchange)
	tableName := fmt.Sprintf("trendlines_%s_%s_%s", strings.ReplaceAll(strings.ToLower(asset), "-", "_"), timeframe, exchange)
	// log.Println(tableName)
	query := fmt.Sprintf(`
		INSERT INTO %s (start_time, start_point, start_inv, start_trendstart, end_time, end_point, end_inv, end_trendstart, direction, status)
		VALUES ($1, $2, $3, $4, $5,$6, $7, $8, $9, $10)
	`, tableName)
	_, err := t.db.Exec(query, tr.Start.Time, tr.Start.Point, tr.Start.Inv, tr.Start.TrendStart, tr.End.Time, tr.End.Point, tr.End.Inv, tr.End.TrendStart, tr.Direction, tr.Status)
	if err != nil {
		log.Printf("Error inserting trendline: %v", err)
		return err
	}
	log.Printf("Trendline inserted successfully")

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
			start_time = $1,
			start_point = $2, 
			start_inv = $3,
			start_trendstart = $4,
			end_time = $5,
			end_point = $6,
			end_inv = $7,
			end_trendstart = $8,
			direction = $9,
			status = $10
		WHERE id = $11
	`, tableName)
	_, err = t.db.Exec(updateQuery, tr.Start.Time, tr.Start.Point, tr.Start.Inv, tr.Start.TrendStart, tr.End.Time, tr.End.Point, tr.End.Inv, tr.End.TrendStart, tr.Direction, tr.Status, id)
	return err
}
