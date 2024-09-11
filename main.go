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
    database, err := db.DBConnect()
    if err != nil {
        log.Fatal("Error on main db connection:", err)
    }
    defer database.Close()

    err = db.CreateTables(database)
    if err != nil {
        log.Fatal("Error creating tables:", err)
    }

    err = db.ListTables(database)
    if err != nil {
        log.Fatal("Error listing tables:", err)
    }

    exchanges, err := db.Get_Exchanges(database)
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
		//http.HandleFunc("/get_candles", handleCandlesRequest)

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
	log.Print("Handle Exchanges Request")

	exchanges, err := api.Get_Exchanges()
	if err != nil {
		log.Printf("Error getting exchanges from API: %v", err)
	}

	jsonData, err := json.Marshal(exchanges)
	if err != nil {
		log.Printf("Error marshalling exchanges: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")

	w.Write(jsonData)



}

/*
func handleCandlesRequest(w http.ResponseWriter, r *http.Request) {
	log.Print("Get Candles Request")

	candles, err := api.Get_Candles()
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
*/


