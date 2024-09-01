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
}


