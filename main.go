package main

import (
	"time"
	"fmt"
	"backend/db"
	"backend/api"
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

	exchanges, _ := db.Get_Exchanges(database)
	if err != nil {
		fmt.Println("Error getting exchanges: ", err)
	}

	for index, _ := range exchanges {
		fmt.Println("\n----------------------------------------\n")
		fmt.Println("ExchangeID: ", exchanges[index].ID)
		fmt.Println("Name: ", exchanges[index].Name)
		fmt.Println("Orders: ", exchanges[index].Orders)
		fmt.Println("Fills: ", exchanges[index].Fills)
		fmt.Println("Watchlist:")
		for wl, _ := range exchanges[index].Watchlist {
			fmt.Println(" : ", exchanges[index].Watchlist[wl])
		}
		fmt.Println("TFs:")
		for tf, _ := range exchanges[index].Timeframes {
			fmt.Println(" : ", exchanges[index].Timeframes[tf])
		}
		fmt.Println("\n----------------------------------------\n")
	}

	defer database.Close()

//	api.ApiConnect()
	//fills := api.Get_Coinbase_Fills()
	//fmt.Println("Fils:")
	//for fill := range fills {
	//	fmt.Println(fills[fill])
	//}

	//orders := api.Get_Coinbase_Orders()

	//fmt.Println("Orders:")
	//for ord := range orders {
	//	fmt.Println()
	//	fmt.Println(orders[ord].Status)
	//	fmt.Println(orders[ord].Side)
	//	fmt.Println(orders[ord].ProductID)
	//	fmt.Println(orders[ord].FilledValue)
	//	fmt.Println(orders[ord].AverageFilledPrice)
	//	fmt.Println(orders[ord].FilledSize)
	//	fmt.Println(orders[ord].OrderType)
	//	fmt.Println(orders[ord].CreatedTime)
	//}
	
	//accounts := api.Get_Coinbase_Account_Balance()
	//fmt.Println("Accounts")
	//for acct := range accounts {
	//	fmt.Println(accounts[acct])
	//}
	productID := "BTC-USD"
	granularity := "ONE_HOUR"
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()

	candles, err := api.Get_Coinbase_Candles(productID, granularity, start, end)
	if err != nil {
		fmt.Printf("Error getting candles: %v\n", err)
		return
	}

	fmt.Println("Candles:")
	fmt.Println(candles)

}


