package main

import (
	"fmt"
	"backend/db"
	"log"

)

func main() {
	fmt.Println("Main ")
	
	// Connect to DB
	database, err := db.DBConnect()
	if err != nil {
		log.Println("Error on main db connection", err)
	}
	err = db.CreateTables(database)
	if err != nil {
		fmt.Println("Error creating tables")
	}
	err = db.ListTables(database)
	if err != nil {
		fmt.Println("Error creating tables")
	}
	defer database.Close()
}

type Product struct {
	Title string
}
type Timeframe struct {
	Label			string
	Tf				int
	Xch				string
}
type Exchange struct {
	Name			string
	Watchlist 		[]Product
	Timeframes 		[]Timeframe
}

func getAccounts() {
}

func getCandles() {
}

