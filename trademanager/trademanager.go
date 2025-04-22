package trademanager

import (
	"backend/common"
	"backend/db"
	"backend/model"
	"database/sql"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/lib/pq"
)

type TradeManager struct {
	db           *sql.DB
	trades       map[int]*model.Trade // Changed to map by trade ID
	tradesMutex  sync.RWMutex
	orderUpdates chan OrderUpdate
	tradeUpdates chan TradeUpdate
	exchanges    map[int]model.ExchangeAPI
}

type TradeState struct {
	GroupID string
	Trades  []model.Trade
}

type OrderUpdate struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

type TradeUpdate struct {
	Trade     model.Trade `json:"trade"`
	Operation string      `json:"operation"` // INSERT, UPDATE, DELETE
}

func NewTradeManager(db *sql.DB, exchanges map[int]model.ExchangeAPI) *TradeManager {
	tm := &TradeManager{
		db:           db,
		trades:       make(map[int]*model.Trade),
		orderUpdates: make(chan OrderUpdate, 100),
		tradeUpdates: make(chan TradeUpdate, 100),
		exchanges:    exchanges,
	}
	return tm
}

func (tm *TradeManager) Initialize() error {
	trades, err := db.GetIncompleteTrades(tm.db)
	if err != nil {
		return err
	}

	tm.tradesMutex.Lock()
	for _, trade := range trades {
		tm.trades[trade.ID] = &trade
	}
	tm.tradesMutex.Unlock()

	go tm.processOrderUpdates()
	go tm.processTradeUpdates()

	return nil
}

func (tm *TradeManager) ListenForDBChanges(dsn string, channel string) {
	listener := pq.NewListener(dsn, 10*time.Second, time.Minute,
		func(ev pq.ListenerEventType, err error) {
			if err != nil {
				log.Printf("TradeManager listener error: %v", err)
			}
		})
	if err := listener.Listen(channel); err != nil {
		log.Fatalf("TradeManager listen error: %v", err)
	}

	log.Printf("TradeManager listening on channel: %s", channel)

	go func() {
		for {
			time.Sleep(30 * time.Second)
			if err := listener.Ping(); err != nil {
				log.Printf("TradeManager error pinging listener: %v", err)
			}
		}
	}()

	for notification := range listener.Notify {
		if notification == nil {
			continue
		}

		var payload struct {
			Table     string          `json:"table"`
			Operation string          `json:"operation"`
			Data      json.RawMessage `json:"data"`
		}

		if err := json.Unmarshal([]byte(notification.Extra), &payload); err != nil {
			log.Printf("TradeManager error parsing notification: %v", err)
			continue
		}

		switch payload.Table {
		case "trades":
			var trade model.Trade
			if err := json.Unmarshal(payload.Data, &trade); err != nil {
				log.Printf("TradeManager error parsing trade data: %v", err)
				continue
			}
			tm.tradeUpdates <- TradeUpdate{
				Trade:     trade,
				Operation: payload.Operation,
			}
		case "orders":
			var order OrderUpdate
			if err := json.Unmarshal(payload.Data, &order); err != nil {
				log.Printf("TradeManager error parsing order data: %v", err)
				continue
			}
			tm.orderUpdates <- order
		case "triggers":
			var trigger common.Trigger
			if err := json.Unmarshal(payload.Data, &trigger); err != nil {
				log.Printf("Error parsing trigger data: %v", err)
				continue
			}
			if trigger.Status == "triggered" {
				tm.handleTriggerUpdate(trigger)
			}
		}
	}
}

func (tm *TradeManager) handleTriggerUpdate(trigger common.Trigger) {
	tradeIDs, err := db.GetTradeIDsForTrigger(tm.db, trigger.ID)
	if err != nil {
		log.Printf("Error getting trades for trigger %d: %v", trigger.ID, err)
		return
	}

	for _, tradeID := range tradeIDs {
		tm.checkAndExecuteTrade(tradeID)
	}
}

func (tm *TradeManager) checkAndExecuteTrade(tradeID int) {
	tm.tradesMutex.RLock()
	trade, exists := tm.trades[tradeID]
	tm.tradesMutex.RUnlock()
	if !exists {
		return
	}

	exchangeAPI, ok := tm.exchanges[trade.XchID]
	if !ok {
		log.Printf("No exchange API found for xch_id %d", trade.XchID)
		return
	}

	triggers, err := db.GetTriggersForTrade(tm.db, tradeID)
	if err != nil {
		log.Printf("Error getting triggers for trade %d: %v", tradeID, err)
		return
	}

	are_all_triggered, err := db.AreAllTriggersTriggered(tm.db, tradeID)
	if err != nil {
		log.Printf("error getting triggers for trade %d: %v", tradeID, err)
	}
	if len(triggers) == 0 || are_all_triggered {
		tm.placeEntryOrder(trade, exchangeAPI)
	}
}

func (tm *TradeManager) processTradeUpdates() {
	for update := range tm.tradeUpdates {
		tm.tradesMutex.Lock()
		trade := update.Trade

		switch update.Operation {
		case "INSERT":
			tm.trades[trade.ID] = &trade
			tm.checkAndExecuteTrade(trade.ID)
		case "UPDATE":
			if existing, exists := tm.trades[trade.ID]; exists {
				*existing = trade
			}
		case "DELETE":
			delete(tm.trades, trade.ID)
		}
		tm.tradesMutex.Unlock()
	}
}

func (tm *TradeManager) placeEntryOrder(trade *model.Trade, exchangeAPI model.ExchangeAPI) {
	orderID, err := exchangeAPI.PlaceOrder(*trade)
	if err != nil {
		log.Printf("TradeManager error placing entry order: %v", err)
		return
	}

	err = db.UpdateTradeEntry(tm.db, trade.ID, orderID)
	if err != nil {
		log.Printf("TradeManager error updating trade entry order: %v", err)
	} else {
		log.Printf("Trade %d executed with order ID %s", trade.ID, orderID)
	}
}

func (tm *TradeManager) processOrderUpdates() {
	for update := range tm.orderUpdates {
		tm.tradesMutex.RLock()
		var targetTrade *model.Trade
		for _, trade := range tm.trades {
			if trade.EntryOrderID == update.OrderID || trade.StopOrderID == update.OrderID || trade.PTOrderID == update.OrderID {
				targetTrade = trade
				break
			}
		}
		tm.tradesMutex.RUnlock()

		if targetTrade == nil {
			continue
		}

		// Look up the exchange API for the trade's XchID
		exchangeAPI, ok := tm.exchanges[targetTrade.XchID]
		if !ok {
			log.Printf("No exchange API found for xch_id %d", targetTrade.XchID)
			continue
		}

		if targetTrade.EntryOrderID == update.OrderID {
			err := db.UpdateTradeEntryStatus(tm.db, targetTrade.ID, update.Status)
			if err != nil {
				log.Printf("TradeManager error updating trade entry status: %v", err)
			}
			if update.Status == "FILLED" && targetTrade.StopOrderID == "" && targetTrade.PTOrderID == "" {
				err = exchangeAPI.PlaceBracketOrder(*targetTrade)
				if err != nil {
					log.Printf("TradeManager error placing bracket orders: %v", err)
				}
			}
		} else if targetTrade.StopOrderID == update.OrderID {
			err := db.UpdateTradeStopStatus(tm.db, targetTrade.ID, update.Status)
			if err != nil {
				log.Printf("TradeManager error updating trade stop status: %v", err)
			}
		} else if targetTrade.PTOrderID == update.OrderID {
			err := db.UpdateTradePTStatus(tm.db, targetTrade.ID, update.Status)
			if err != nil {
				log.Printf("TradeManager error updating trade pt status: %v", err)
			}
		}

		if targetTrade.EntryStatus == "FILLED" && (targetTrade.StopStatus == "FILLED" || targetTrade.PTStatus == "FILLED") {
			err := db.UpdateTradeStatusByID(tm.db, targetTrade.ID, "completed")
			if err != nil {
				log.Printf("TradeManager error updating trade status for %d: %v", targetTrade.ID, err)
			}
		}
	}
}
