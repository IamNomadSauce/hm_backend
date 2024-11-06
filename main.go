package main

import (
	"backend/api"
	"backend/db"
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
	DB *sql.DB
	mu sync.Mutex
}

var app *Application

func main() {
	fmt.Println("Main 10.25.2024")

	// Connect to DB
	database, err := db.DBConnect()
	if err != nil {
		log.Fatal("Error on main db connection:", err)
	}

	app = &Application{
		DB: database,
	}
	// defer app.DB.Close()

	// err = db.CreateTables(app.DB)
	// if err != nil {
	// 	log.Fatal("Error creating tables:", err)
	// }

	err = db.ListTables(app.DB)
	if err != nil {
		log.Fatal("Error listing tables:", err)
	}

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
					log.Printf("API for exchange %s is not initialized", exchange.Name)
					continue
				}
				err := api.Fetch_And_Store_Candles(exchange, app.DB, false)
				if err != nil {
					log.Printf("Error fetching and storing candles for %s: %v\n", exchange.Name, err)
				}

				// for _, product := range available_products {
				// 	fmt.Println("Product: ", product)
				// }
				// fmt.Println("Available Products: ", len(available_products))
				// fills, err := exchange.API.FetchFills()
				// if err != nil {
				// 	log.Printf("Error fetching fills for %s: %v", exchange.Name, err)
				// } else {
				// 	log.Printf("%s Fills: %d", exchange.Name, len(fills))
				// }

			}
			time.Sleep(1 * time.Minute)
		}
	}()

	go func() {
		log.Println("Starting HTTP server goroutine")
		http.HandleFunc("/", handleMain)
		http.HandleFunc("/exchanges", handleExchangesRequest)
		http.HandleFunc("/candles", handleCandlesRequest)

		// TODO Make App config struct and add DB
		log.Println("Server starting on :31337")
		err := http.ListenAndServe(":31337", nil)
		if err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	select {}
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

	jsonData, err := json.Marshal(exchanges)
	if err != nil {
		log.Printf("Error marshalling exchanges: %v", err)
	}

	for _, exchange := range exchanges {
		fmt.Println("\n----------------------\nExchange: ", exchange.Name)
		fmt.Println("Watchlist: ", exchange.Watchlist)
		fmt.Println("Timeframes: ", exchange.Timeframes)
		fmt.Println("\n--------------------------\n")
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
