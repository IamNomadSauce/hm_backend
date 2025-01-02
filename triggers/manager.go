package triggers

import (
	"backend/common"
	"sync"
)

func NewTriggerManager() *TriggerManager {
	return &TriggerManager{
		triggers: make(map[string][]common.Trigger),
	}
}

type TriggerManager struct {
	triggers     map[string][]common.Trigger
	triggerMutex sync.RWMutex
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
	tm.triggerMutex.RLock()
	defer tm.triggerMutex.RUnlock()

	var triggeredTriggers []common.Trigger
	if triggers, exists := tm.triggers[productID]; exists {
		for _, trigger := range triggers {
			if trigger.Status != "active" {
				continue
			}

			if trigger.Type == "price_above" && price > trigger.Price || trigger.Type == "price_below" && price < trigger.Price {
				trigger.Status = "triggered"
				triggeredTriggers = append(triggeredTriggers, trigger)

			}
		}
	}
	return triggeredTriggers
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
