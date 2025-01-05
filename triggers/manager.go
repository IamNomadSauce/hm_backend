package triggers

import (
	"backend/common"
	"database/sql"
	"fmt"
	"log"
	"sync"
)

func NewTriggerManager(db *sql.DB) *TriggerManager {
	return &TriggerManager{
		triggers: make(map[string][]common.Trigger),
		db:       db,
	}
}

type TriggerManager struct {
	triggers     map[string][]common.Trigger
	triggerMutex sync.RWMutex
	db           *sql.DB
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

func (tm *TriggerManager) UpdateTriggers(triggers []common.Trigger) {
	tm.triggerMutex.Lock()
	defer tm.triggerMutex.Unlock()

	newTriggers := make(map[string][]common.Trigger)
	for _, trigger := range triggers {
		newTriggers[trigger.ProductID] = append(newTriggers[trigger.ProductID], trigger)
	}
	tm.triggers = newTriggers
}

func (tm *TriggerManager) ProcessPriceUpdate(productID string, price float64) []common.Trigger {
	// log.Printf("\nTM:ProcessPriceUpdate:\n Product: %s\n Price: %f", productID, price)
	tm.triggerMutex.RLock()
	defer tm.triggerMutex.RUnlock()
	// log.Println("TM.ProcessPriceUpdate:Triggers:", tm.triggers)

	var triggeredTriggers []common.Trigger
	if triggers, exists := tm.triggers[productID]; exists {
		for _, trigger := range triggers {
			if trigger.ProductID != productID || trigger.Status != "active" {
				continue
			}

			// log.Printf("\nTrigger:\n Product: %s\n Type: %s\n Trigger-Price: %f\n Price: %f", trigger.ProductID, trigger.Type, trigger.Price, price)

			var triggered bool
			switch trigger.Type {
			case "price_below":
				triggered = price < trigger.Price
			case "price_above":
				triggered = price > trigger.Price
				if price > trigger.Price {
					trigger.Status = "triggered"
				}
			}

			if triggered {
				log.Printf("TRIGGERED: %s\n %f Above %f\n", trigger.ProductID, price, trigger.Price)

				trigger.Status = "triggered"
				if err := tm.updateTriggerStatus(trigger.ID, "triggered"); err != nil {
					log.Printf("Error updating trigger status: %v", err)
					continue
				}
				triggeredTriggers = append(triggeredTriggers, trigger)
			}
		}
	}
	return triggeredTriggers
}

func (tm *TriggerManager) updateTriggerStatus(triggerID int, status string) error {
	query := `
		UPDATE triggers
		SET status = $1,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
		RETURNING id
	`

	var updatedID int
	err := tm.db.QueryRow(query, status, triggerID).Scan(&updatedID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("trigger with ID %d not found", triggerID)
	}
	return err
}

func GetTriggers(db *sql.DB, xch_id int, status string) ([]common.Trigger, error) {
	query := `
        SELECT 
            id, product_id, type, price, timeframe, 
            candle_count, condition, status, triggered_count,
            xch_id, created_at, updated_at
        FROM triggers
    `

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

func (tm *TriggerManager) ProcessCandleUpdate(productID string, timeframe string, candle common.Candle) []common.Trigger {
	tm.triggerMutex.RLock()
	defer tm.triggerMutex.RUnlock()

	var triggeredTriggers []common.Trigger
	if triggers, exists := tm.triggers[productID]; exists {
		for _, trigger := range triggers {
			if trigger.Status != "active" || trigger.Timeframe != timeframe {
				continue
			}

			triggered := false
			switch trigger.Condition {
			case "WICKS_ABOVE":
				triggered = candle.High > trigger.Price
			case "WICKS_BELOW":
				triggered = candle.Low < trigger.Price
			case "CLOSES_ABOVE":
				triggered = candle.Close > trigger.Price
			case "CLOSES_BELOW":
				triggered = candle.Close < trigger.Price
			}

			if triggered {
				trigger.TriggeredCount++
				if trigger.TriggeredCount >= trigger.CandleCount {
					trigger.Status = "triggered"
					triggeredTriggers = append(triggeredTriggers, trigger)
				}
			}
		}
	}
	return triggeredTriggers
}
