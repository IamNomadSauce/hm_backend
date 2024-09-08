package main

import (
	"fmt"
	"backend/db"
	"backend/api"
	_"backend/model"
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

