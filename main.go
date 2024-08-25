package main

import (
	"fmt"
)

func main() {
	fmt.Println("Main ")

	// main loop
	var wg sync.WaitGroup
	mainLoop := func() {
		wg.Add(1)
		go func() {
			wg.Done()
		}
	}
	
	ticker := time.NewTicker(60 * time.Second)
	go func() {
		for range ticker.C {
			mainLoop()
		}
	}

	// Get Watchlist
	// for each product in watchlist
		// get candles for each timeframe 
		// write candles to db
	// for each exchange
		// get account 
			// get portfolio
			// get fills
			// get orders
	
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
	Name:			string
	Watchlist 		[]Product
	Timeframes 		[]Timeframe
}

func getAccounts() {
}

func getCandles() {
}

