package indicators

import (
	"backend/common"
	"backend/indicators/trendlines"
	"backend/sse"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
)

type Indicator interface {
	ProcessCandle(asset, timeframe, exchange string, candle common.Candle) error
	Start() error
}

type Indicators struct {
	db         *sql.DB
	dsn        string
	assets     []string
	timeframes []string
	exchanges  []string
	indicators map[string][]Indicator
	// triggerMgr *triggers.TriggerManager
	// tradeMgr   *trademanager.TradeManager
	ssemanager *sse.SSEManager
	mutex      sync.RWMutex
}

// func NewIndicators(db *sql.DB, dsn string, assets, timeframes []string, triggerMgr *triggers.TriggerManager, tradeMgr *trademanager.TradeManager) *Indicators {
func NewIndicatorManager(db *sql.DB, dsn string, assets []string, timeframes []string, exchanges []string, sseManager *sse.SSEManager) *Indicators {
	log.Println("\n---------------------\nNew Indicator Manager")

	im := &Indicators{
		db:         db,
		dsn:        dsn,
		assets:     assets,
		timeframes: timeframes,
		exchanges:  exchanges,
		indicators: make(map[string][]Indicator),
		ssemanager: sseManager,
		// triggerMgr: triggerMgr,
		// tradeMgr:   tradeMgr,
	}

	im.registerIndicators()
	return im
}

func (i *Indicators) registerIndicators() {
	log.Println("Register Indicators")
	for _, asset := range i.assets {
		for _, tf := range i.timeframes {
			for _, exchange := range i.exchanges {
				asset = strings.Replace(asset, "-", "_", -1)
				indicator := trendlines.NewTrendlineIndicator(i.db, i.ssemanager, asset, tf, exchange)
				i.RegisterIndicator(asset, tf, exchange, indicator)
				i.ssemanager.BroadcastMessage(fmt.Sprintf("New Indicator %s %s %s", asset, tf, exchange))
			}
		}
	}
}

func (i *Indicators) RegisterIndicator(asset, timeframe, exchange string, indicator Indicator) {
	log.Println("Register Indicator", asset, timeframe)
	i.mutex.Lock()
	defer i.mutex.Unlock()
	key := fmt.Sprintf("%s_%s_%s", strings.ToLower(asset), timeframe, exchange)
	log.Println("InidatorKey", key)
	i.indicators[key] = append(i.indicators[key], indicator)
	log.Printf("Indicators: %+v", i.indicators)

	if err := indicator.Start(); err != nil {
		log.Printf("Error starting indicator for %s %s %s: %v", asset, timeframe, exchange, err)
	}
}

func (i *Indicators) Start() error {
	log.Println("Indicator.Start")

	if err := i.ensureNotifyFunctionExists(); err != nil {
		return fmt.Errorf("error ensuring notify function exists: %w", err)
	}

	if err := i.attachTriggersToCandlestickTables(); err != nil {
		return fmt.Errorf("error attaching triggers: %w", err)
	}

	go i.listenForCandleUpdates()

	return nil
}
func (i *Indicators) ensureNotifyFunctionExists() error {
	var exists bool
	query := `
        SELECT EXISTS (
            SELECT 1
            FROM pg_proc
            WHERE proname = 'notify_candle_update'
        )`
	err := i.db.QueryRow(query).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check for notify_candle_update function: %w", err)
	}

	if !exists {
		createFunctionQuery := `
            CREATE OR REPLACE FUNCTION notify_candle_update() RETURNS trigger AS $$
            BEGIN
                PERFORM pg_notify('candle_updates', json_build_object(
                    'table', TG_TABLE_NAME,
                    'operation', TG_OP,
                    'data', row_to_json(NEW)
                )::text);
                RETURN NEW;
            END;
            $$ LANGUAGE plpgsql;`
		_, err := i.db.Exec(createFunctionQuery)
		if err != nil {
			return fmt.Errorf("failed to create notify_candle_update function: %w", err)
		}
		log.Println("Created notify_candle_update function")
	} else {
		log.Println("notify_candle_update function already exists")
	}

	return nil
}

func (i *Indicators) attachTriggersToCandlestickTables() error {
	log.Println("Attach Triggers To Candlestick Tables")
	for _, exchange := range i.exchanges {
		for _, asset := range i.assets {
			sanitizedAsset := strings.ReplaceAll(strings.ToLower(asset), "-", "_")
			for _, tf := range i.timeframes {
				tableName := fmt.Sprintf("%s_%s_%s", sanitizedAsset, tf, exchange)
				log.Println("TableName", tableName)
				triggerName := fmt.Sprintf("notify_candle_update_%s", tableName)
				query := fmt.Sprintf(`
					DO $$
					BEGIN
						IF NOT EXISTS (
							SELECT 1
							FROM information_schema.triggers
							WHERE event_object_table = '%s'
							AND trigger_name = '%s'
						) THEN
							CREATE TRIGGER %s
							AFTER INSERT OR UPDATE ON %s
							FOR EACH ROW EXECUTE FUNCTION notify_candle_update();
						END IF;
					END;
					$$;`, tableName, triggerName, triggerName, tableName)
				_, err := i.db.Exec(query)
				if err != nil {
					return fmt.Errorf("failed to create trigger for %s: %w", tableName, err)
				}
				log.Printf("Attached trigger to %s", tableName)
			}
		}
	}
	return nil
}
func (i *Indicators) listenForCandleUpdates() {
	log.Println("Indicators: Listen For Candle Updates")
	listener := pq.NewListener(i.dsn, 10*time.Second, time.Minute, func(ev pq.ListenerEventType, err error) {
		if err != nil {
			log.Printf("Listener error: %v", err)
		}
	})
	if err := listener.Listen("candle_updates"); err != nil {
		log.Fatalf("Error listening to channel: %v", err)
	}

	log.Println("Indicators listening for candle updates...")
	for {
		select {
		case notification := <-listener.Notify:
			if notification != nil {
				var payload struct {
					Table     string          `json:"table"`
					Operation string          `json:"operation"`
					Data      json.RawMessage `json:"data"`
				}
				if err := json.Unmarshal([]byte(notification.Extra), &payload); err != nil {
					log.Printf("Error parsing notification: %v", err)
					continue
				}

				// log.Printf("----------------\nPayload.Data\n-------------------")
				// log.Printf("%s", payload.Data) // Log as string for readability
				// log.Printf("-----------------------------------")

				// Parse table name to get asset, timeframe, and exchange
				asset, timeframe, exchange, err := parseTableName(payload.Table)
				if err != nil {
					log.Printf("Invalid table name %s: %v", payload.Table, err)
					continue
				}

				// Temporary struct to match database JSON structure
				type RawCandle struct {
					Timestamp int64   `json:"timestamp"` // Assuming numeric timestamp
					Open      float64 `json:"open"`      // Numeric fields from DB
					High      float64 `json:"high"`
					Low       float64 `json:"low"`
					Close     float64 `json:"close"`
					Volume    float64 `json:"volume"`
					ProductID string  `json:"product_id,omitempty"` // Optional
				}

				var raw RawCandle
				if err := json.Unmarshal(payload.Data, &raw); err != nil {
					log.Printf("Error parsing raw candle data: %v", err)
					continue
				}

				// Map RawCandle to common.Candle
				candle := common.Candle{
					ProductID: raw.ProductID,
					Timestamp: raw.Timestamp,
					Open:      raw.Open,
					High:      raw.High,
					Low:       raw.Low,
					Close:     raw.Close,
					Volume:    raw.Volume,
				}

				// If ProductID is not in the JSON, set it from asset
				if candle.ProductID == "" {
					candle.ProductID = strings.ToUpper(asset) // e.g., "XLM-USD"
				}

				i.processCandle(asset, timeframe, exchange, candle)
			}
		case <-time.After(60 * time.Second):
			if err := listener.Ping(); err != nil {
				log.Printf("Ping failed: %v", err)
			}
		}
	}
}

func parseTableName(tableName string) (string, string, string, error) {
	// log.Println("Parse Table Name", tableName)
	parts := strings.Split(tableName, "_")
	if len(parts) < 3 {
		return "", "", "", fmt.Errorf("invalid table name format")
	}
	// log.Println("Parts:", parts)
	asset := strings.Join(parts[:len(parts)-1], "_")
	timeframe := parts[len(parts)-2]
	exchange := parts[len(parts)-1]
	return strings.ToLower(asset), timeframe, strings.ToLower(exchange), nil
}

func (i *Indicators) processCandle(asset, timeframe, exchange string, candle common.Candle) {
	// log.Println("\n----------\nprocessCandle", asset, timeframe, candle)
	i.mutex.RLock()
	defer i.mutex.RUnlock()

	key := fmt.Sprintf("%s_%s_%s", strings.ToLower(asset), timeframe, exchange)
	// log.Println("Indicator Key", key)
	// log.Printf("Indicators %+v", i.indicators)
	if indicators, exists := i.indicators[key]; exists {
		log.Println("\n----------------------\nIndicator Exists\n------------------------")
		// log.Printf("Indicator", i.indicators[key])
		// log.Printf("%+v", indicators)
		for _, ind := range indicators {
			if err := ind.ProcessCandle(asset, timeframe, exchange, candle); err != nil {
				log.Printf("Error processing candle for %s %s %s: %v", asset, timeframe, exchange, err)
			}
		}
	}
}
