package main

import (
	"fmt"
	"backend/db"
	"log"

)

func main() {
	fmt.Println("Main ")
	
	// Connect to DB
	_, err := db.DBConnect()
	if err != nil {
		log.Println("Error on main db connection", err)
	}



	// main loop
//	var wg sync.WaitGroup
//	mainLoop := func() {
//		wg.Add(1)
//		go func() {
//			wg.Done()
//		}
//	}
//	
//	ticker := time.NewTicker(60 * time.Second)
//	go func() {
//		for range ticker.C {
//			mainLoop()
//		}
//	}

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

