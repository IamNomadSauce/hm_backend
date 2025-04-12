package triggers

import (
	"backend/common" // Adjust import path as needed
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
)

type Trigger interface {
	// IsTriggered(candles []common.Candle) bool
	// GetTradeAction() TradeAction
	// GetID() int
	// GetAction() string
}

type TradeAction struct {
	ProductID string
	Side      string
}

func NewTriggerManager(db *sql.DB) *TriggerManager {
	return &TriggerManager{
		db:            db,
		triggers:      make(map[string][]common.Trigger),
		candleHistory: make(map[string][]common.Candle),
		maxHistory:    100,
	}
}

// TriggerManager manages trigger-related operations
type TriggerManager struct {
	db            *sql.DB
	triggers      map[string][]common.Trigger
	triggerMutex  sync.RWMutex
	candleHistory map[string][]common.Candle
	maxHistory    int
}

func (tm *TriggerManager) InitializeTriggersFromExchange(exchangeID int) error {
	log.Println("TM.InitializeTriggersFromExchange", exchangeID)
	triggers, err := GetTriggers(tm.db, exchangeID, "active")
	if err != nil {
		return fmt.Errorf("failed to initialize triggers: %w", err)
	}

	tm.triggerMutex.Lock()
	defer tm.triggerMutex.Unlock()

	for _, trigger := range triggers {
		tm.triggers[trigger.ProductID] = append(tm.triggers[trigger.ProductID], trigger)
	}
	return nil

}

// GetTriggersForTrade retrieves triggers associated with a trade
func (tm *TriggerManager) GetTriggersForTrade(tradeID int) ([]common.Trigger, error) {
	query := `
        SELECT t.id, t.product_id, t.type, t.price, t.timeframe, 
               t.candle_count, t.condition, t.status, t.triggered_count, 
               t.xch_id, t.created_at, t.updated_at
        FROM triggers t
        JOIN trade_triggers tt ON t.id = tt.trigger_id
        WHERE tt.trade_id = $1`
	rows, err := tm.db.Query(query, tradeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var triggers []common.Trigger
	for rows.Next() {
		var t common.Trigger
		if err := rows.Scan(&t.ID, &t.ProductID, &t.Type, &t.Price, &t.Timeframe,
			&t.CandleCount, &t.Condition, &t.Status, &t.TriggeredCount,
			&t.XchID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		triggers = append(triggers, t)
	}
	return triggers, nil
}

func GetTriggers(db *sql.DB, xch_id int, status string) ([]common.Trigger, error) {
	query := `
		SELECT id, product_id, type, price, timeframe, candle_count, condition,
			status, triggered_count, xch_id, created_at, updated_at 
		FROM triggers    `

	var rows *sql.Rows
	var err error
	if status != "" {
		query += " WHERE status = $1 AND xch_id = $2"
		rows, err = db.Query(query, status, xch_id)
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

// AreAllTriggersTriggered checks if all triggers for a trade are triggered
func (tm *TriggerManager) AreAllTriggersTriggered(tradeID int) (bool, error) {
	query := `
        SELECT COUNT(*)
        FROM trade_triggers tt
        JOIN triggers t ON tt.trigger_id = t.id
        WHERE tt.trade_id = $1 AND t.status != 'triggered'`
	var count int
	err := tm.db.QueryRow(query, tradeID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// UpdateTradeStatusByID updates the status of a trade
func (tm *TriggerManager) UpdateTradeStatusByID(tradeID int, status string) error {
	query := `UPDATE trades SET status = $1 WHERE id = $2`
	_, err := tm.db.Exec(query, status, tradeID)
	return err
}

// ProcessCandleUpdate processes candle updates and returns triggered triggers
func (tm *TriggerManager) ProcessCandleUpdate(productID string, timeframe string, candle common.Candle) []common.Trigger {
	// log.Printf("Process Candle Update: %s %s %+v", productID, timeframe, candle)
	tm.triggerMutex.RLock()
	defer tm.triggerMutex.RUnlock()

	key := productID + "_" + timeframe
	if _, ok := tm.candleHistory[key]; !ok {
		tm.candleHistory[key] = []common.Candle{}
	}
	tm.candleHistory[key] = append(tm.candleHistory[key], candle)
	if len(tm.candleHistory[key]) > tm.maxHistory {
		tm.candleHistory[key] = tm.candleHistory[key][1:]
	}

	productID = strings.ToUpper(productID)
	productID = strings.ReplaceAll(productID, "_", "-")
	// for i, _ := range tm.triggers {
	// 	log.Printf("productID: %s\ntriggerID: %s", productID, i)
	// 	if productID == i {
	// 		log.Println("Matched")

	// 	}
	// }

	var triggeredTriggers []common.Trigger
	if triggers, exists := tm.triggers[productID]; exists {
		fmt.Printf("\n---------------------------\nTrigger Exists: %s\n", productID)
		for i, trigger := range triggers {
			// log.Printf("Trigger: %+v\n", trigger)
			if trigger.Status == "active" {
				if tm.checkCandleCondition(trigger, key) {
					log.Println("\n--------------TRIGGERED--------", trigger, key)
					trigger.Status = "triggered"
					trigger.TriggeredCount = trigger.CandleCount
					if err := tm.updateTriggerStatus(trigger.ID, "triggerd"); err != nil {
						log.Printf("Error updating trigger status: %v", err)
						continue
					}
					triggeredTriggers = append(triggeredTriggers, trigger)

					trades, err := tm.GetTriggersForTrade(trigger.ID)
					if err != nil {
						log.Printf("Error getting triggers for trade: %v", err)
						continue
					}
					for _, trade := range trades {
						allTriggered, err := tm.AreAllTriggersTriggered(trade.ID)
						if err != nil {
							log.Printf("Error checking triggers for trade %d: %v", trade.ID, err)
							continue
						}
						if allTriggered {
							if err := tm.UpdateTradeStatusByID(trade.ID, "ready_for_entry"); err != nil {
								log.Printf("Error updating trade status: %v", err)
							}
						}
					}
					triggers[i] = trigger
				}
			}
			if trigger.Status != "active" || trigger.Timeframe != timeframe {
				// fmt.Printf("TRIGGER:============> %s %s", trigger, productID)
				continue
			}

		}
		tm.triggers[productID] = triggers
	} else {
		// log.Printf("No triggers available for %s", productID)
	}
	return triggeredTriggers
}

func (tm *TriggerManager) checkCandleCondition(trigger common.Trigger, historyKey string) bool {
	fmt.Printf("Check Candle Condition for Trigger: %v\n", trigger.ID)
	fmt.Printf("%s: ", trigger.Timeframe)
	fmt.Printf("%s: ", trigger.Status)
	fmt.Printf("Trigger: %s", trigger.ProductID)
	fmt.Printf("\t%s", trigger.Type)
	fmt.Printf(" %f", trigger.Price)
	fmt.Printf("\t%s", trigger.Timeframe)
	fmt.Printf("\nCandle_Count %d", trigger.CandleCount)
	fmt.Printf("\tTrigger_Count %d", trigger.TriggeredCount)
	// fmt.Println("\n========================================\n")

	// history, ok := tm.candleHistory[historyKey]
	// if !ok || len(history) < trigger.CandleCount {
	// 	return false
	// }

	// lastNCandles := history[len(history)-trigger.CandleCount:]

	fmt.Printf("\n\nTRIGGER_CONDITION: |%s|\n", trigger.Type)
	switch trigger.Type {
	case "closes_above":
		fmt.Println("CLOSES_ABOVE\n")
		// for _, c := range lastNCandles {
		// 	if c.Close <= trigger.Price {
		// 		return false
		// 	}
		// }
		fmt.Printf("\n------------------------------------\n")
		return false
	case "closes_below":
		fmt.Println("CLOSES_BELOW\n")
		// for _, c := range lastNCandles {
		// 	if c.Close >= trigger.Price {
		// 		return false
		// 	}
		// }
		fmt.Printf("\n------------------------------------\n")
		return false
	case "price_above":
		fmt.Println("PRICE_ABOVE\n")
		// for _, c := range lastNCandles {
		// 	if c.High <= trigger.Price {
		// 		return false
		// 	}
		// }
		fmt.Printf("\n------------------------------------\n")
		return false
	case "price_below":
		fmt.Println("PRICE_BELOW\n")
		// for _, c := range lastNCandles {
		// 	if c.Close >= trigger.Price {
		// 		return false
		// 	}
		// }
		fmt.Printf("\n------------------------------------\n")
		return false
	default:
		// log.Printf("Unsupported trigger condition: %s", trigger)
		return false
	}

}

// ProcessPriceUpdate processes price updates and returns triggered triggers
func (tm *TriggerManager) ProcessPriceUpdate(productID string, price float64) []common.Trigger {
	tm.triggerMutex.RLock()
	defer tm.triggerMutex.RUnlock()

	var triggeredTriggers []common.Trigger
	if triggers, exists := tm.triggers[productID]; exists {
		for i, trigger := range triggers {
			if trigger.ProductID != productID || trigger.Status != "active" {
				continue
			}

			var triggered bool
			switch trigger.Type {
			case "price_below":
				triggered = price < trigger.Price
			case "price_above":
				triggered = price > trigger.Price
			}

			if triggered {
				trigger.Status = "triggered"
				if err := tm.updateTriggerStatus(trigger.ID, "triggered"); err != nil {
					log.Printf("Error updating trigger status: %v", err)
					continue
				}
				triggeredTriggers = append(triggeredTriggers, trigger)

				query := `
                    SELECT trade_id
                    FROM trade_triggers
                    WHERE trigger_id = $1`
				rows, err := tm.db.Query(query, trigger.ID)
				if err != nil {
					log.Printf("Error querying associated trades: %v", err)
					continue
				}
				defer rows.Close()

				for rows.Next() {
					var tradeID int
					if err := rows.Scan(&tradeID); err != nil {
						log.Printf("Error scanning trade ID: %v", err)
						continue
					}

					allTriggered, err := tm.AreAllTriggersTriggered(tradeID)
					if err != nil {
						log.Printf("Error checking triggers for trade %d: %v", tradeID, err)
						continue
					}

					if allTriggered {
						if err := tm.UpdateTradeStatusByID(tradeID, "ready_for_entry"); err != nil {
							log.Printf("Error updating trade status: %v", err)
						}
					}
				}
				triggers[i] = trigger
			}
		}
		tm.triggers[productID] = triggers
	}
	return triggeredTriggers
}

// Helper methods (assumed to exist or can be added)
func (tm *TriggerManager) updateTriggerStatus(triggerID int, status string) error {
	query := `UPDATE triggers SET status = $1 WHERE id = $2`
	_, err := tm.db.Exec(query, status, triggerID)
	return err
}

func (tm *TriggerManager) updateTriggerCount(triggerID int, count int) error {
	query := `UPDATE triggers SET triggered_count = $1 WHERE id = $2`
	_, err := tm.db.Exec(query, count, triggerID)
	return err
}

func (tm *TriggerManager) UpdateTriggers(triggers []common.Trigger) {
	tm.triggerMutex.Lock()
	defer tm.triggerMutex.Unlock() // Update only the specific triggers while preserving others
	for _, trigger := range triggers {
		// Get current triggers for this product
		currentTriggers := tm.triggers[trigger.ProductID]

		// Update or append the trigger
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
		// log.Printf("Trigger: %+v\n", trigger)
	}
	// log.Printf("Trigger: %+v", tm.triggers)
	return nil
}

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

	var messages []common.Trigger
	for rows.Next() {
		var alert common.Trigger
		err := rows.Scan(
			&alert.ID,
			&alert.ProductID,
			&alert.Type,
			&alert.Price,
			&alert.Timeframe,
			&alert.CandleCount,
			&alert.Condition,
			&alert.Status,
			&alert.TriggeredCount,
			&alert.XchID,
			&alert.CreatedAt,
			&alert.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, alert)
	}
	return messages, nil

}
