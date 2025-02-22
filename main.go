package main

import (
	"backend/api"
	"backend/common"
	"backend/db"
	"backend/model"
	_ "backend/model"
	"backend/sse"
	"backend/trademanager"
	"backend/triggers"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Application struct {
	DB             *sql.DB
	mu             sync.Mutex
	Exchanges      map[int]*model.Exchange
	TriggerManager *triggers.TriggerManager
	SSEManager     *sse.SSEManager
	TradeManager   *trademanager.TradeManager
}

var app *Application

func main() {

	app = &Application{
		Exchanges: make(map[int]*model.Exchange), // Initialize the map
	}

	// Connect to DB
	database, err := db.DBConnect()
	if err != nil {
		log.Fatal("Error on main db connection:", err)
	}
	app.DB = database // Set the DB connection

	// Initialize trigger manager
	triggers_manager := triggers.NewTriggerManager(app.DB)
	app.TriggerManager = triggers_manager
	app.SSEManager = sse.NewSSEManager(triggers_manager)
	app.TradeManager = trademanager.NewTradeManager(app.DB)

	if err := app.TradeManager.Initialize(); err != nil {
		log.Fatalf("Error initializing TradeManager: %v", err)
	}

	initialProduct := "XLM-USD"

	// Now you can safely get exchanges using app.DB
	db_exchanges, err := db.Get_Exchanges(app.DB)
	if err != nil {
		log.Printf("Error getting initial exchanges: %v", err)
	} else {
		// Initialize websockets
		for _, exchange := range db_exchanges {

			if err := triggers_manager.InitializeTriggersFromExchange(exchange.ID); err != nil {
				log.Printf("Error initializing triggers for exchange %s: %v", exchange.Name, err)
			}

			if exchange.API == nil {
				log.Printf("API for exchange %s is not initialized\n", exchange.Name)
				continue
			}

			exchange.API.SetManagers(triggers_manager, app.SSEManager)

			if err := exchange.API.ConnectMarketDataWebSocket(); err != nil {
				log.Printf("Error connecting Market WebSocket for %s: %v", exchange.Name, err)
			}

			if err := exchange.API.ConnectUserWebsocket(); err != nil {
				log.Printf("Error connecting WebSocket for %s: %v", exchange.Name, err)
			}

		}
		log.Println("Websockets Initialized")
	}

	defer app.DB.Close()

	err = db.CreateTables(app.DB)
	if err != nil {
		log.Fatal("Error creating tables:", err)
	}

	err = db.ListTables(app.DB)
	if err != nil {
		log.Fatal("Error listing tables:", err)
	}

	// Fill db loop
	go func() {
		for {
			db_exchanges, err := db.Get_Exchanges(app.DB)
			if err != nil {
				log.Printf("Error getting exchanges: %v", err)
				time.Sleep(1 * time.Minute)
				continue
			}
			for _, exchange := range db_exchanges {
				if exchange.API == nil {
					// log.Printf("API for exchange %s is not initialized\n", exchange.Name)
					continue
				}
				err := api.Fetch_And_Store_Candles(exchange, app.DB, false)
				if err != nil {
					log.Printf("Error fetching and storing candles for %s: %v\n", exchange.Name, err)
				}

				err = api.Do_AvailableProducts(exchange, app.DB)
				if err != nil {
					log.Printf("error executing Do_AvailableProducts for %s\n%v\n", exchange.Name, err)
				}

				err = api.Do_Orders_and_Fills(exchange, app.DB)
				if err != nil {
					log.Printf("Error executing Do_Orders for %s\n%v\n", exchange.Name, err)
				}

				// log.Println("Do_Portfolio")
				err = api.Do_Portfolio(exchange, app.DB)
				if err != nil {
					log.Printf("Error executing Do_Portfolio for %s\n%v", exchange.Name, err)
				}
			}
			time.Sleep(1 * time.Minute)
		}
	}()

	host := os.Getenv("PG_HOST")
	portStr := os.Getenv("PG_PORT")
	user := os.Getenv("PG_USER")
	password := os.Getenv("PG_PASS")
	dbname := os.Getenv("PG_DBNAME")

	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Errorf("Invalid port number: %v", err)
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	go app.SSEManager.ListenForDBChanges(dsn, "global_changes", initialProduct)
	go app.TradeManager.ListenForDBChanges(dsn, "global_changes")

	// Web Server
	go func() {
		log.Println("Starting HTTP server goroutine")
		http.HandleFunc("/", handleMain)
		http.HandleFunc("/exchanges", handleExchangesRequest)
		http.HandleFunc("/candles", handleCandlesRequest)
		http.HandleFunc("/add-to-watchlist", addToWatchlistHandler)
		http.HandleFunc("/new_trade", TradeBlockHandler)
		http.HandleFunc("/delete-trade-group", deleteTradeBlockHandler)
		http.HandleFunc("/create-trigger", createTriggerHandler)
		http.HandleFunc("/delete-trigger", deleteTriggerHandler)
		http.HandleFunc("/update-trigger", updateTriggerHandler)
		http.HandleFunc("/cancel-order", cancelOrderHandler)

		http.Handle("/trigger/stream", app.SSEManager)

		// TODO Make App config struct and add DB
		log.Println("Server starting on :31337")
		err := http.ListenAndServe(":31337", nil)
		if err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Old Trade Manager
	// go func() {
	// 	log.Println("startingTrade Manage goroutine")
	// 	for {
	// 		trades, err := db.GetAllTrades(app.DB)
	// 		if err != nil {
	// 			log.Printf("Error getting incomplete trades: %v", err)
	// 			continue
	// 		}

	// 		tradeGroups := make(map[string][]model.Trade)
	// 		for _, trade := range trades {
	// 			tradeGroups[trade.GroupID] = append(tradeGroups[trade.GroupID], trade)
	// 		}

	// 		// log.Println("Trade Blocks: ", len(tradeGroups))
	// 		for groupID, groupTrades := range tradeGroups {
	// 			// log.Printf("Processing trade group: %s", groupID)

	// 			for _, trade := range groupTrades {
	// 				exchange, err := db.Get_Exchange(trade.XchID, database)
	// 				if err != nil {
	// 					log.Println("Error getting exchange: ", err)
	// 					continue
	// 				}

	// 				// Only place new orders if there's no entry order ID and status is empty
	// 				if trade.EntryOrderID == "" && trade.EntryStatus == "" {
	// 					log.Printf("Placing entry order for trade in group %s", groupID)
	// 					log.Printf("Entry: %f\nEntry: %f\nEntry: %f\n",
	// 						trade.EntryPrice, trade.StopPrice, trade.PTPrice)

	// 					orderID, err := exchange.API.PlaceOrder(trade)
	// 					if err != nil {
	// 						log.Printf("Error placing entry order: %v", err)
	// 						continue
	// 					}

	// 					err = db.UpdateTradeEntry(database, trade.ID, orderID)
	// 					if err != nil {
	// 						log.Printf("Error updating trade entry order: %v", err)
	// 					}
	// 				} else if trade.EntryOrderID != "" {
	// 					// Check existing order status
	// 					order, err := exchange.API.GetOrder(trade.EntryOrderID)
	// 					if err != nil {
	// 						log.Printf("Error checking order status: %v", err)
	// 						continue
	// 					}

	// 					if order.Status != trade.EntryStatus {
	// 						err = db.UpdateTradeStatus(database, trade.GroupID, order.Status, trade.StopStatus, trade.PTStatus)
	// 						if err != nil {
	// 							log.Printf("Error updating trade status: %v", err)
	// 						}
	// 					}

	// 					// Place bracket orders only when entry is filled and no brackets exist
	// 					if order.Status == "FILLED" && trade.StopOrderID == "" && trade.PTOrderID == "" {
	// 						log.Printf("Placing bracket orders for filled entry in group %s", groupID)
	// 						err := exchange.API.PlaceBracketOrder(trade)
	// 						if err != nil {
	// 							log.Printf("Error placing bracket orders: %v", err)
	// 						}
	// 					}
	// 				}
	// 			}
	// 		}

	// 		time.Sleep(1 * time.Minute)
	// 	}
	// }()
	// select {}
	select {}
}

func cancelOrderHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Cancel Order Handler")

	if r.Method != http.MethodPost {
		log.Println("Method Not Allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		OrderID string `json:"order_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Println("OrderID", request.OrderID)

	if request.OrderID == "" {
		log.Println("Invalid Order ID")
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	// Get the exchange from the order
	order, err := db.Get_Order(request.OrderID, app.DB)
	if err != nil {
		log.Printf("Error getting order: %v", err)
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	log.Printf("Order: %+v", order)

	exchange, err := db.Get_Exchange(order.XchID, app.DB)
	if err != nil {
		log.Printf("Error getting exchange: %v", err)
		http.Error(w, "Exchange not found", http.StatusNotFound)
		return
	}

	// Cancel the order using exchange API
	err = exchange.API.CancelOrder(request.OrderID)
	if err != nil {
		log.Printf("Error canceling order: %v", err)
		http.Error(w, "Failed to cancel order", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Order cancelled successfully",
	})
}

func updateTriggerHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Update Trigger")

	if r.Method != http.MethodPut {
		log.Println("Method Not Allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Match the frontend structure
	var request struct {
		TriggerID int                    `json:"trigger_id"`
		Updates   map[string]interface{} `json:"updates"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Trigger to be updated\n%+v", request)

	// Extract values from updates map
	updates := request.Updates
	query := `
        UPDATE triggers 
        SET 
            type = COALESCE($1, type),
            price = COALESCE($2, price),
            timeframe = COALESCE($3, timeframe),
            candle_count = COALESCE($4, candle_count),
            condition = COALESCE($5, condition),
            status = COALESCE($6, status),
            updated_at = CURRENT_TIMESTAMP
        WHERE id = $7
        RETURNING id`

	var updatedID int
	err := app.DB.QueryRow(
		query,
		getStringValue(updates, "type"),
		getFloatValue(updates, "price"),
		getStringValue(updates, "timeframe"),
		getIntValue(updates, "candles"), // Note: frontend uses "candles" instead of "candle_count"
		getStringValue(updates, "condition"),
		getStringValue(updates, "status"),
		request.TriggerID,
	).Scan(&updatedID)

	if err == sql.ErrNoRows {
		log.Printf("Trigger not found: %d", request.TriggerID)
		http.Error(w, "Trigger not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("Error updating trigger: %v", err)
		http.Error(w, "Failed to update trigger", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Trigger updated successfully",
	})
}

// Helper functions to safely extract values from the updates map
func getStringValue(updates map[string]interface{}, key string) interface{} {
	if val, ok := updates[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return nil
}

func getFloatValue(updates map[string]interface{}, key string) interface{} {
	if val, ok := updates[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		}
	}
	return nil
}

// Add/modify these helper functions in your backend
func getIntValue(updates map[string]interface{}, key string) interface{} {
	if val, ok := updates[key]; ok {
		switch v := val.(type) {
		case float64:
			return int(v) // Convert float64 to int
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		case int:
			return v
		}
	}
	return nil
}

// For debugging, add this before the SQL query
// log.Printf("Updating candle_count with value: %v", getIntValue(updates, "candles"))

// Helper functions for handling null values
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullIfZero(f float64) interface{} {
	if f == 0 {
		return nil
	}
	return f
}

func deleteTriggerHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Delete Trigger")

	if r.Method != http.MethodDelete {
		log.Println("Method Not Allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		TriggerID int `json:"trigger_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Println("Error decoding request into struct")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	log.Println("Payload", request)
	log.Println("TriggerID:", request.TriggerID)

	if request.TriggerID <= 0 {
		log.Println("Invalid Trigger ID")
		http.Error(w, "Invalid trigger ID", http.StatusBadRequest)
		return
	}

	if err := db.DeleteTrigger(app.DB, request.TriggerID); err != nil {
		log.Printf("Error deleting trigger: %v", err)
		http.Error(w, "Failed to delete trigger", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Trigger deleted successfully",
	})
}

func createTriggerHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Create Trigger")

	var trigger common.Trigger

	if err := json.NewDecoder(r.Body).Decode(&trigger); err != nil {
		log.Println("Error decoding json for new trigger")
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if trigger.ProductID == "" || trigger.Type == "" || (trigger.Type != "price_above" && trigger.Type != "price_below") || trigger.Price <= 0 {
		http.Error(w, "Invalid input: ProductID, Type (price_above/price_below, and Price are required", http.StatusBadRequest)
		return
	}

	triggerID, err := db.CreateTrigger(app.DB, &trigger)
	if err != nil {
		log.Println("Error inserting trigger into database:", err)
		http.Error(w, fmt.Sprintf("Failed to create trigger: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "applilcation/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"message":   "trigger Created Successfuly",
		"triggerID": triggerID,
	})
}

func TradeBlockHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Place Bracket Order")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var trade_group model.TradeBlock

	// var request struct {
	// 	ProductID     string    `json:"product_id"`
	// 	Side          string    `json:"side"`
	// 	Size          float64   `json:"size"`
	// 	EntryPrice    float64   `json:"entry_price"`
	// 	StopPrice     float64   `json:"stop_price"`
	// 	ProfitTargets []float64 `json:"profit_targets"`
	// 	RiskReward    float64   `json:"risk_reward"`
	// 	ExchangeID    int       `json:"xch_id"`
	// }

	if err := json.NewDecoder(r.Body).Decode(&trade_group); err != nil {
		log.Println("Error decoding json for new trade group")
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Trade-Group:\n%s %s\n|Entry: %f\n|Amount: %f\n|StopLoss: %f\n|Risk:Reward %f",
		trade_group.ProductID,
		trade_group.Side,
		trade_group.EntryPrice,
		trade_group.Size,
		trade_group.StopPrice,
		trade_group.Triggers,
		// trade_group.RiskReward,
	)

	exchange, err := db.Get_Exchange(trade_group.XchID, app.DB)
	if err != nil {
		log.Printf("error getting exchange: %v", err)
		http.Error(w, "Exchange not found", http.StatusNotFound)
		return
	}

	// log.Println("Exchange: ", exchange)

	var trades []model.Trade
	base_size := trade_group.Size / float64(len(trade_group.ProfitTargets))
	log.Println("Trade Block split into", len(trade_group.ProfitTargets), "orders of base_size:", base_size)
	for _, pt := range trade_group.ProfitTargets {

		var trade model.Trade
		trade.Side = trade_group.Side
		trade.ProductID = trade_group.ProductID
		trade.Size = base_size
		trade.EntryPrice = trade_group.EntryPrice
		trade.StopPrice = trade_group.StopPrice
		trade.PTPrice = pt
		trade.XchID = exchange.ID
		trades = append(trades, trade)
	}

	var triggerIDs []int
	for _, trigger := range trade_group.Triggers {
		triggerIDs = append(triggerIDs, trigger.ID)
	}

	err = db.WriteTrades(app.DB, trades, triggerIDs)
	if err != nil {
		log.Printf("Error writing to trades: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// for _, trade := range trades {
	// 	err = exchange.API.PlaceBracketOrder(trade)
	// 	if err != nil {
	// 		log.Printf("Error placing bracket order: %v", err)
	// 		http.Error(w, fmt.Sprintf("Failed to place bracket order: %v", err), http.StatusInternalServerError)
	// 		return
	// 	}
	// }

	w.Header().Set("Content-Type", "applilcation/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Bracket order placed successfully",
	})
}

func deleteTradeBlockHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Delete Trade Block Handler")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		GroupID string `json:"group_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := db.DeleteTradeGroup(app.DB, request.GroupID); err != nil {
		log.Printf("Error deleting trade group: %v", err)
		http.Error(w, "Failed to delete trade group", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Trade group deleted successfully",
	})
}

//

func addToWatchlistHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("AddToWatchlistHandler")

	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var request struct {
		ExchangeID int    `json:"xch_id"`
		ProductID  string `json:"product_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Request: exchange_id=%d, product_id=%s", request.ExchangeID, request.ProductID)

	// Use the db package function
	err := db.Write_Watchlist(app.DB, request.ExchangeID, request.ProductID)
	if err != nil {
		log.Printf("Error writing to watchlist: %v", err)
		http.Error(w, "Failed to add to watchlist", http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Product added to watchlist",
	})
}

func handleMain(w http.ResponseWriter, r *http.Request) {
	log.Print("Connected to Main")

	w.Write([]byte("HElo world"))
}

func handleExchangesRequest(w http.ResponseWriter, r *http.Request) {
	log.Print("\n-----------------------------------\n Handle Exchanges Request \n-----------------------------------\n")

	exchanges, err := api.Get_Exchanges(app.DB)
	if err != nil {
		log.Printf("Error getting exchanges from API: %v", err)
	}
	/*
		log.Print("\n-------------------------\nhandleExchangesRequest:Exchanges", exchanges[0].Name)
		log.Print("\n-------------------------\nhandleExchangesRequest:Exchanges", exchanges[0].Timeframes)
		log.Print("\n-------------------------\nhandleExchangesRequest:Exchanges", exchanges[0].Watchlist)
	*/
	// log.Print("\n-------------------------\nhandleExchangesRequest:Exchanges", exchanges[0].AvailableProducts)

	// for _, exchange := range exchanges {
	// 	log.Println("\n----------------------\nExchange: ", exchange.Name)
	// 	log.Println("Watchlist: ", exchange.Watchlist)
	// 	log.Println("Timeframes: ", exchange.Timeframes)
	// 	log.Println("Available Products: ", len(exchange.AvailableProducts))
	// 	log.Println("\n--------------------------\n")
	// }
	jsonData, err := json.Marshal(exchanges)
	if err != nil {
		log.Printf("Error marshalling exchanges: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")

	// log.Println("WRITING EXCAHGES")
	w.Write(jsonData)
}

func handleCandlesRequest(w http.ResponseWriter, r *http.Request) {
	log.Print("\n-----------------------------\n Get Candles Request \n-----------------------------\n")

	product := r.URL.Query().Get("product")
	timeframe := r.URL.Query().Get("timeframe")
	exchange := r.URL.Query().Get("exchange")

	log.Printf("Request:main: %s_%s_%s", product, timeframe, exchange)

	// Update SSE manager's selected product
	tableName := strings.ToLower(fmt.Sprintf("%s_%s_%s",
		strings.ReplaceAll(product, "-", "_"),
		timeframe,
		exchange))

	// Get the global SSE manager and update its listening table
	if app.SSEManager != nil {
		app.SSEManager.UpdateSelectedProduct(tableName, product)
	}

	candles, err := api.Get_Candles(product, timeframe, exchange, app.DB)
	if err != nil {
		log.Printf("Error getting candles handleCandlesRequest %v", err)
		return
	}

	jsonData, err := json.Marshal(candles)
	if err != nil {
		fmt.Printf("Error marshalling candles %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}
