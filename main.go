package main

import (
	"backend/api"
	"backend/db"
	_ "backend/model"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	fmt.Println("Main ")

	// Connect to DB
	database, err := db.DBConnect()
	if err != nil {
		log.Fatal("Error on main db connection:", err)
	}
	defer database.Close()

	err = db.CreateTables()
	if err != nil {
		log.Fatal("Error creating tables:", err)
	}

	err = db.ListTables()
	if err != nil {
		log.Fatal("Error listing tables:", err)
	}

	go func() {
		for {
			db_exchanges, err := db.Get_Exchanges(database)
			if err != nil {
				log.Fatal("Error getting exchanges:", err)
			}
			for _, exchange := range db_exchanges {
				err := api.Fetch_And_Store_Candles(exchange, database)
				if err != nil {
					log.Printf("Error fetching and storing candles for %s: %v\n", exchange.Name, err)
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

		log.Println("Server starting on :31337")
		err := http.ListenAndServe(":31337", nil)
		if err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	select {}
}

func fetchDataForExchanges(exchanges []Exchange) {
	for _, exchange := range exchanges {

		orders, err := exchange.GetOrders()
		if err != nil {
			log.Printf("Error fetching orders: %v", err)
		}
		fmt.Println("Orders:", orders)

		fills, err := exchange.GetFills()
		if err != nil {
			log.Printf("Error fetching orders: %v", err)
		}
		fmt.Println("Fills:", fills)

		timeframes, err := exchange.GetTimeframes()
		if err != nil {
			log.Printf("Error fetching orders: %v", err)
		}
		fmt.Println("Timeframes:", timeframes)

		watchlist, err := exchange.GetWatchlist()
		if err != nil {
			log.Printf("Error fetching watchlist: %v", err)
		}
		fmt.Println("Watchlist:", watchlist)

		portfolio, err := exchange.GetPortfolio()
		if err != nil {
			log.Printf("Error fetching portfolio: %v", err)
		}
		fmt.Println("Portfolio:", portfolio)

		candles, err := exchange.GetCandles()
		if err != nil {
			log.Printf("Error fetching candles: %v", err)
		}
		fmt.Println("Candles:", candles)
	}
}

func handleMain(w http.ResponseWriter, r *http.Request) {
	log.Print("Connected to Main")

	w.Write([]byte("HElo world"))
}

func handleExchangesRequest(w http.ResponseWriter, r *http.Request) {
	log.Print("\n-----------------------------------\n Handle Exchanges Request \n-----------------------------------\n")

	exchanges, err := api.Get_Exchanges()
	if err != nil {
		log.Printf("Error getting exchanges from API: %v", err)
	}
	/*
		log.Print("\n-------------------------\nhandleExchangesRequest:Exchanges", exchanges[0].Name)
		log.Print("\n-------------------------\nhandleExchangesRequest:Exchanges", exchanges[0].Timeframes)
		log.Print("\n-------------------------\nhandleExchangesRequest:Exchanges", exchanges[0].Watchlist)
	*/

	jsonData, err := json.Marshal(exchanges)
	if err != nil {
		log.Printf("Error marshalling exchanges: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")

	w.Write(jsonData)
}

func handleCandlesRequest(w http.ResponseWriter, r *http.Request) {
	log.Print("\n-----------------------------\n Get Candles Request \n-----------------------------\n")

	product := r.URL.Query().Get("product")
	timeframe := r.URL.Query().Get("timeframe")
	exchange := r.URL.Query().Get("exchange")

	log.Printf("Request:main: %s_%s_%s", product, timeframe, exchange)

	candles, err := api.Get_Candles(product, timeframe, exchange)
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
