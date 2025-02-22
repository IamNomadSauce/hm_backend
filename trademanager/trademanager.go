package trademanager

import (
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
	trades       map[string]*TradeState
	tradesMutex  sync.RWMutex
	orderUpdates chan OrderUpdate
	tradeUpdates chan TradeUpdate
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

func NewTradeManager(db *sql.DB) *TradeManager {
	tm := &TradeManager{
		db:           db,
		trades:       make(map[string]*TradeState),
		orderUpdates: make(chan OrderUpdate, 100),
		tradeUpdates: make(chan TradeUpdate, 100),
	}
	return tm
}

func (tm *TradeManager) Initialize() error {
	trades, err := db.GetIncompleteTrades(tm.db)
	if err != nil {
		return err
	}

	tm.tradesMutex.Lock()
	defer tm.tradesMutex.Unlock()

	for _, trade := range trades {
		if _, exists := tm.trades[trade.GroupID]; !exists {
			tm.trades[trade.GroupID] = &TradeState{
				GroupID: trade.GroupID,
				Trades:  []model.Trade{},
			}
		}
		tm.trades[trade.GroupID].Trades = append(tm.trades[trade.GroupID].Trades, trade)
	}

	go tm.ListenForDBChanges("your_dsn", "global_changes")

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

	// Ping routine to keep connection alive
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
		}
	}
}

func (tm *TradeManager) processTradeUpdates() {
	for update := range tm.tradeUpdates {
		tm.tradesMutex.Lock()
		trade := update.Trade
		groupID := trade.GroupID

		switch update.Operation {
		case "INSERT":
			if _, exists := tm.trades[groupID]; !exists {
				tm.trades[groupID] = &TradeState{GroupID: groupID,
					Trades: []model.Trade{},
				}
			}
			tm.trades[groupID].Trades = append(tm.trades[groupID].Trades, trade)

			if trade.EntryOrderID == "" && trade.EntryStatus == "" {
				tm.placeEntryOrder(&trade)
			}
		case "UPDATE":
			if tradeState, exists := tm.trades[groupID]; exists {
				for i, t := range tradeState.Trades {
					if t.ID == trade.ID {
						tradeState.Trades[i] = trade
						break
					}
				}
			}
		case "DELETE":
			if tradeState, exists := tm.trades[groupID]; !exists {
				for i, t := range tradeState.Trades {
					if t.ID == trade.ID {
						tradeState.Trades = append(tradeState.Trades[:i], tradeState.Trades[i+1:]...)
						if len(tradeState.Trades) == 0 {
							delete(tm.trades, groupID)
						}
						break
					}
				}
			}
		}
		tm.tradesMutex.Unlock()
	}
}

func (tm *TradeManager) placeEntryOrder(trade *model.Trade) {
	exchange, err := db.Get_Exchange(trade.XchID, tm.db)
	if err != nil {
		log.Printf("TradeManager error placing entry order: %v", err)
		return
	}

	orderID, err := exchange.API.PlaceOrder(*trade)
	if err != nil {
		log.Printf("TradeManager error placing entry order: %v", err)
		return
	}

	err = db.UpdateTradeEntry(tm.db, trade.ID, orderID)
	if err != nil {
		log.Printf("TradeManager error updating trade entry order: %v", err)
	}
}

func (tm *TradeManager) processOrderUpdates() {
	for update := range tm.orderUpdates {
		tm.tradesMutex.RLock()

		var targetTrade *model.Trade
		var targetGroupID string
		for groupID, tradeState := range tm.trades {
			for _, trade := range tradeState.Trades {
				if trade.EntryOrderID == update.OrderID || trade.StopOrderID == update.OrderID || trade.PTOrderID == update.OrderID {
					targetTrade = &trade
					targetGroupID = groupID
					break
				}
			}
			if targetTrade != nil {
				break
			}
		}
		tm.tradesMutex.RUnlock()

		if targetTrade == nil {
			continue
		}

		if targetTrade.EntryOrderID == update.OrderID {
			err := db.UpdateTradeStatus(tm.db, targetTrade.GroupID, update.Status, targetTrade.StopStatus, targetTrade.PTStatus)
			if err != nil {
				log.Printf("TradeManager error updating trade status: %v", err)
			}

			if update.Status == "FILLED" && targetTrade.StopOrderID == "" && targetTrade.PTOrderID == "" {
				exchange, err := db.Get_Exchange(targetTrade.XchID, tm.db)
				if err != nil {
					log.Printf("TradeManager error getting exchange: %v", err)
					continue
				}

				err = exchange.API.PlaceBracketOrder(*targetTrade)
				if err != nil {
					log.Printf("TradeManager error placing bracket orders: %v", err)
				}
			}
		}
	}
}
