package main

import (
	"fmt"
	"backend/db"
	"backend/api"
	_"backend/model"
	"log"
	"encoding/json"
	"net/http"
	"time"

)

func main() {
    fmt.Println("Main ")

    // Connect to DB
    err := db.DBConnect()
    if err != nil {
        log.Fatal("Error on main db connection:", err)
    }
    defer db.DB.Close()

    err = db.CreateTables()
    if err != nil {
        log.Fatal("Error creating tables:", err)
    }

    err = db.ListTables()
    if err != nil {
        log.Fatal("Error listing tables:", err)
    }

    exchanges, err := db.Get_Exchanges()
    if err != nil {
        log.Fatal("Error getting exchanges:", err)
    }
    log.Println("Exchanges", exchanges)

    go func() {
        if len(exchanges) > 0 {
            coinbase := exchanges[0]
            for {
                err = api.Fill_Exchange(coinbase, false)
                if err != nil {
                    log.Println("Error Fill exchange:", err)
                }
                time.Sleep(1 * time.Minute)
            }
        } else {
            log.Println("No exchanges found")
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
	select{}
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



















