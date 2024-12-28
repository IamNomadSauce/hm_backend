package main

import (
	"backend/alerts"
	"backend/api"
	"backend/db"
	"backend/model"
	_ "backend/model"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type Application struct {
	DB           *sql.DB
	mu           sync.Mutex
	Exchanges    map[int]*model.Exchange
	AlertManager *alerts.AlertManager
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

	// Initialize alert manager
	alerts_manager := alerts.NewAlertManager()
	app.AlertManager = alerts_manager

	// Now you can safely get exchanges using app.DB
	db_exchanges, err := db.Get_Exchanges(app.DB)
	if err != nil {
		log.Printf("Error getting initial exchanges: %v", err)
	} else {
		// Initialize websockets
		for _, exchange := range db_exchanges {
			if exchange.API == nil {
				log.Printf("API for exchange %s is not initialized\n", exchange.Name)
				continue
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
					log.Printf("error executing Do_AvailableProducts for %s\n%w\n", exchange, err)
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

	go func() {
		log.Println("Starting HTTP server goroutine")
		http.HandleFunc("/", handleMain)
		http.HandleFunc("/exchanges", handleExchangesRequest)
		http.HandleFunc("/candles", handleCandlesRequest)
		http.HandleFunc("/add-to-watchlist", addToWatchlistHandler)
		http.HandleFunc("/new_trade_group", TradeBlockHandler)
		http.HandleFunc("/create-alert", createAlertHandler)

		// TODO Make App config struct and add DB
		log.Println("Server starting on :31337")
		err := http.ListenAndServe(":31337", nil)
		if err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Trade Manager
	go func() {
		log.Println("startingTrade Manage goroutine")
		for {
			trades, err := db.GetAllTrades(app.DB)
			if err != nil {
				log.Printf("Error getting incomplete trades: %v", err)
				continue
			}

			tradeGroups := make(map[string][]model.Trade)
			for _, trade := range trades {
				tradeGroups[trade.GroupID] = append(tradeGroups[trade.GroupID], trade)
			}

			log.Println("Trade Blocks: ", len(tradeGroups))
			for groupID, groupTrades := range tradeGroups {
				log.Printf("Processing trade group: %s", groupID)

				for _, trade := range groupTrades {
					exchange, err := db.Get_Exchange(trade.XchID, database)
					if err != nil {
						log.Println("Error getting exchange: ", err)
						continue
					}

					// Only place new orders if there's no entry order ID and status is empty
					if trade.EntryOrderID == "" && trade.EntryStatus == "" {
						log.Printf("Placing entry order for trade in group %s", groupID)
						log.Printf("Entry: %f\nEntry: %f\nEntry: %f\n",
							trade.EntryPrice, trade.StopPrice, trade.PTPrice)

						orderID, err := exchange.API.PlaceOrder(trade)
						if err != nil {
							log.Printf("Error placing entry order: %v", err)
							continue
						}

						err = db.UpdateTradeEntry(database, trade.ID, orderID)
						if err != nil {
							log.Printf("Error updating trade entry order: %v", err)
						}
					} else if trade.EntryOrderID != "" {
						// Check existing order status
						order, err := exchange.API.GetOrder(trade.EntryOrderID)
						if err != nil {
							log.Printf("Error checking order status: %v", err)
							continue
						}

						if order.Status != trade.EntryStatus {
							err = db.UpdateTradeStatus(database, trade.GroupID, order.Status, trade.StopStatus, trade.PTStatus)
							if err != nil {
								log.Printf("Error updating trade status: %v", err)
							}
						}

						// Place bracket orders only when entry is filled and no brackets exist
						if order.Status == "FILLED" && trade.StopOrderID == "" && trade.PTOrderID == "" {
							log.Printf("Placing bracket orders for filled entry in group %s", groupID)
							err := exchange.API.PlaceBracketOrder(trade)
							if err != nil {
								log.Printf("Error placing bracket orders: %v", err)
							}
						}
					}
				}
			}

			time.Sleep(1 * time.Minute)
		}
	}()

	// Alerts. not so sure about this
	// go func() {
	// 	log.Println("Starting Alert Manager GoRoutine")
	// 	ticker := time.NewTicker(1 * time.Minute)
	// 	defer ticker.Stop()

	// 	for {
	// 		select {
	// 		case <-ticker.C:

	// 			// alerts, err := app.DB.GetActiveAlerts(app.DB)
	// 			// if err != nil {
	// 			// 	log.Printf("Error getting alerts: %v", err)
	// 			// 	continue
	// 			// }
	// 			// app.AlertManager.UpdateAlerts(alerts)
	// 		}
	// 	}
	// }()

	select {}
}

func createAlertHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Create Alert")

	var alert alerts.Alert

	if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
		log.Println("Error decoding json for new alert")
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if alert.ProductID == "" || alert.Type == "" || (alert.Type != "price_above" && alert.Type != "price_below") || alert.Price <= 0 {
		http.Error(w, "Invalid input: ProductID, Type (price_above/price_below, and Price are required", http.StatusBadRequest)
		return
	}

	alertID, err := db.CreateAlert(app.DB, &alert)
	if err != nil {
		log.Println("Error inserting alert into database:", err)
		http.Error(w, fmt.Sprintf("Failed to create alert: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "applilcation/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Alert Created Successfuly",
		"alertID": alertID,
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

	err = db.WriteTrades(app.DB, trades)
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
