package main

import (
	_"time"
	"fmt"
	"backend/db"
	"backend/api"
	_"backend/model"
	"log"
	"time"

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

	coinbase := exchanges[0]
	for {
		err = api.Fill_Exchange(coinbase, false)
		if err != nil {
			fmt.Println("Error Fill exchange", err)
		}

		time.Sleep(1 * time.Minute)
	}

	defer database.Close()

	api.ApiConnect()

}

