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
		xch := exchanges[index]
		fmt.Println("ExchangeID: ", xch.ID)
		fmt.Println("Name: ", xch.Name)
		fmt.Println("Orders: ", xch.Orders)
		fmt.Println("Fills: ", xch.Fills)
		fmt.Println("Watchlist:")
		for wl, _ := range xch.Watchlist {
			prod := xch.Watchlist[wl].Product
			fmt.Println(" : ", prod)
			for tf, _ := range xch.Timeframes {
				timeframe := xch.Timeframes[tf]
				endpoint := timeframe.Endpoint
				minutes := timeframe.Minutes
				end := time.Now()
				start := end.Add(-time.Duration(minutes * 300) * time.Minute)
				candles, err := api.Get_Coinbase_Candles(prod, endpoint, start, end)
				if err != nil {
					fmt.Println(err)
				}
				fmt.Println(prod, endpoint, len(candles))
			}
		}
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

}


