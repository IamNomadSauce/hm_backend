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
	log.Println("\n---------------------\nNew Indicator Manager\n")

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
	for _, asset := range i.assets {
		for _, tf := range i.timeframes {
			for _, exchange := range i.exchanges {
				indicator := trendlines.NewTrendlineIndicator(i.db, i.ssemanager)
				i.RegisterIndicator(asset, tf, exchange, indicator)
				i.ssemanager.BroadcastMessage(fmt.Sprintf("New Indicator %s %s", asset, tf, exchange))
			}
		}
	}
}

func (i *Indicators) RegisterIndicator(asset, timeframe, exchange string, indicator Indicator) {
	log.Println("Register Indicator", asset, timeframe)
	i.mutex.Lock()
	defer i.mutex.Unlock()
	key := fmt.Sprintf("%s_%s", strings.ToLower(asset), timeframe)
	log.Println("InidatorKey", key)
	i.indicators[key] = append(i.indicators[key], indicator)
	log.Printf("Indicators: %+v", i.indicators)
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

				asset, timeframe, exchange, err := parseTableName(payload.Table)
				if err != nil {
					log.Printf("Invalid table name %s: %v", payload.Table, err)
					continue
				}

				var candle common.Candle
				if err := json.Unmarshal(payload.Data, &candle); err != nil {
					log.Printf("Error parsing candle data: %v", err)
					continue
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
	log.Println("Parse Table Name", tableName)
	parts := strings.Split(tableName, "_")
	if len(parts) < 3 {
		return "", "", "", fmt.Errorf("invalid table name format")
	}
	log.Println("Parts:", parts)
	asset := strings.Join(parts[:len(parts)-1], "_")
	timeframe := parts[len(parts)-2]
	exchange := parts[len(parts)-1]
	return strings.ToLower(asset), timeframe, strings.ToLower(exchange), nil
}

func (i *Indicators) processCandle(asset, timeframe, exchange string, candle common.Candle) {
	log.Println("processCandle", asset, timeframe, candle)
	i.mutex.RLock()
	defer i.mutex.RUnlock()

	key := fmt.Sprintf("%s_%s", strings.ToLower(asset), timeframe)
	if indicators, exists := i.indicators[key]; exists {
		for _, ind := range indicators {
			if err := ind.ProcessCandle(asset, timeframe, exchange, candle); err != nil {
				log.Printf("Error processing candle for %s %s: %v")
			}
		}
	}
}
