package triggers

import (
	"backend/common"
	"database/sql"
	"log"
	"sync"
	"fmt"
)

// TriggerManager manages trigger-related operations
type TriggerManager struct {
	db            *sql.DB
	triggers      map[string][]common.Trigger
	triggerMutex  sync.RWMutex
	candleHistory map[string][]common.Candle
	maxHistory    int
}

// NewTriggerManager initializes a new TriggerManager instance
func NewTriggerManager(db *sql.DB) *TriggerManager {
	return &TriggerManager{
		db:            db,
		triggers:      make(map[string][]common.Trigger),
		candleHistory: make(map[string][]common.Candle),
		maxHistory:    100,
	}
}

// InitializeTriggersFromExchange loads active triggers for an exchange
func (tm *TriggerManager) InitializeTriggersFromExchange(exchangeID int) error {
	log.Println("TM.InitializeTriggersFromExchange", exchangeID)
	triggers, err := GetTriggers(tm.db, exchangeID, "active")
	if err != nil {
		return err // Error message already formatted in GetTriggers
	}

	tm.triggerMutex.Lock()
	defer tm.triggerMutex.Unlock()

	for _, trigger := range triggers {
		tm.triggers[trigger.ProductID] = append(tm.triggers[trigger.ProductID], trigger)
	}
	return nil
}

// GetTriggers retrieves triggers from the database
func GetTriggers(db *sql.DB, xchID int, status string) ([]common.Trigger, error) {
	query := `
        SELECT id, product_id, type, price, timeframe, candle_count, condition,
            status, triggered_count, xch_id, created_at, updated_at 
        FROM triggers`
	var rows *sql.Rows
	var err error
	if status != "" {
		query += " WHERE status = $1 AND xch_id = $2"
		rows, err = db.Query(query, status, xchID)
	} else {
		rows, err = db.Query(query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var triggersList []common.Trigger
	for rows.Next() {
		var trigger common.Trigger
		err := rows.Scan(
			&trigger.ID,
			&trigger.ProductID,
			&trigger.Type,
			&trigger.Price,
			&trigger.Timeframe,
			&trigger.CandleCount,
			&trigger.Condition,
			&trigger.Status,
			&trigger.TriggeredCount,
			&trigger.XchID,
			&trigger.CreatedAt,
			&trigger.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		triggersList = append(triggersList, trigger)
	}
	return triggersList, nil
}

// ProcessCandleUpdate processes candle updates and returns triggered triggers
func (tm *TriggerManager) ProcessCandleUpdate(product, timeframe string, candle common.Candle) []common.Trigger {
	fmt.Printf("Process Candle Update, %s %s\n", product, timeframe)
	tm.triggerMutex.Lock()
	defer tm.triggerMutex.Unlock()

	key := product + "_" + timeframe

	// Update candle history
	if _, ok := tm.candleHistory[key]; !ok {
		tm.candleHistory[key] = []common.Candle{}
	}
	tm.candleHistory[key] = append(tm.candleHistory[key], candle)
	if len(tm.candleHistory[key]) > tm.maxHistory {
		tm.candleHistory[key] = tm.candleHistory[key][1:]
	}

	var triggered []common.Trigger
	triggers, exists := tm.triggers[product]
	if !exists {
		return triggered
	}

	for i, trigger := range triggers {
		if trigger.Timeframe != timeframe || trigger.Status != "active" {
			continue
		}
		if tm.checkCandleCondition(trigger, key) {
			fmt.Printf("Candle Condition Met %+v %s", trigger, key)
			trigger.Status = "triggered"
			triggers[i] = trigger
			triggered = append(triggered, trigger)
			if err := tm.updateTriggerStatus(trigger.ID, "triggered"); err != nil {
				log.Printf("Error updating trigger %d status: %v", trigger.ID, err)
				continue
			}
		}
	}
	tm.triggers[product] = triggers
	return triggered
}

// ProcessPriceUpdate processes price updates and returns triggered triggers
func (tm *TriggerManager) ProcessPriceUpdate(productID string, price float64) []common.Trigger {
	tm.triggerMutex.Lock()
	defer tm.triggerMutex.Unlock()

	var triggeredTriggers []common.Trigger
	if triggers, exists := tm.triggers[productID]; exists {
		for i, trigger := range triggers {
			if trigger.Status != "active" {
				continue
			}

			var triggered bool
			switch trigger.Type {
			case "price_below":
				triggered = price < trigger.Price
			case "price_above":
				triggered = price > trigger.Price
			default:
				continue
			}

			if triggered {
				trigger.Status = "triggered"
				if err := tm.updateTriggerStatus(trigger.ID, "triggered"); err != nil {
					log.Printf("Error updating trigger %d status: %v", trigger.ID, err)
					continue
				}
				triggeredTriggers = append(triggeredTriggers, trigger)
				triggers[i] = trigger
			}
		}
		tm.triggers[productID] = triggers
	}
	return triggeredTriggers
}

// checkCandleCondition checks if a trigger condition is met based on candle data
func (tm *TriggerManager) checkCandleCondition(trigger common.Trigger, historyKey string) bool {
	fmt.Printf("Check Candle Condition %s", historyKey)
	history, ok := tm.candleHistory[historyKey]
	if !ok || len(history) < trigger.CandleCount {
		return false
	}

	lastNCandles := history[len(history)-trigger.CandleCount:]

	switch trigger.Type {
	case "closes_above":
		for _, c := range lastNCandles {
			if c.Close <= trigger.Price {
				return false
			}
		}
		return true
	case "closes_below":
		for _, c := range lastNCandles {
			if c.Close >= trigger.Price {
				return false
			}
		}
		return true
	case "price_above":
		for _, c := range lastNCandles {
			if c.High <= trigger.Price {
				return false
			}
		}
		return true
	case "price_below":
		for _, c := range lastNCandles {
			if c.Low >= trigger.Price {
				return false
			}
		}
		return true
	default:
		log.Printf("Unsupported trigger type: %s", trigger.Type)
		return false
	}
}

// updateTriggerStatus updates the status of a trigger in the database
func (tm *TriggerManager) updateTriggerStatus(triggerID int, status string) error {
	query := `UPDATE triggers SET status = $1 WHERE id = $2`
	_, err := tm.db.Exec(query, status, triggerID)
	return err
}

// UpdateTriggers updates the in-memory trigger list
func (tm *TriggerManager) UpdateTriggers(triggers []common.Trigger) {
	tm.triggerMutex.Lock()
	defer tm.triggerMutex.Unlock()

	for _, trigger := range triggers {
		currentTriggers := tm.triggers[trigger.ProductID]
		updated := false
		for i, existing := range currentTriggers {
			if existing.ID == trigger.ID {
				currentTriggers[i] = trigger
				updated = true
				break
			}
		}
		if !updated {
			currentTriggers = append(currentTriggers, trigger)
		}
		tm.triggers[trigger.ProductID] = currentTriggers
	}
}

// ReloadTriggers reloads all active triggers from the database
func (tm *TriggerManager) ReloadTriggers() error {
	log.Println("tm: Reload Triggers")
	tm.triggerMutex.Lock()
	defer tm.triggerMutex.Unlock()

	tm.triggers = make(map[string][]common.Trigger)
	triggers, err := GetActiveTriggers(tm.db)
	if err != nil {
		return err
	}
	for _, trigger := range triggers {
		tm.triggers[trigger.ProductID] = append(tm.triggers[trigger.ProductID], trigger)
	}
	return nil
}

// GetActiveTriggers retrieves all active triggers from the database
func GetActiveTriggers(db *sql.DB) ([]common.Trigger, error) {
	query := `
        SELECT id, product_id, type, price, timeframe, candle_count, 
            condition, status, triggered_count, xch_id, created_at, 
            updated_at 
        FROM triggers 
        WHERE status = 'active' 
        ORDER BY id ASC`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var triggers []common.Trigger
	for rows.Next() {
		var trigger common.Trigger
		err := rows.Scan(
			&trigger.ID,
			&trigger.ProductID,
			&trigger.Type,
			&trigger.Price,
			&trigger.Timeframe,
			&trigger.CandleCount,
			&trigger.Condition,
			&trigger.Status,
			&trigger.TriggeredCount,
			&trigger.XchID,
			&trigger.CreatedAt,
			&trigger.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		triggers = append(triggers, trigger)
	}
	return triggers, nil
}
