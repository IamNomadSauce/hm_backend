package api

import (
	"backend/db"
	"backend/model"
	"database/sql"
	_ "database/sql"
	"fmt"
	"log"
	"time"
)

func ApiConnect() {
	fmt.Println("API Connected")
}

// TODO
// Get all (populated) exchanges for sending to frontend client
func Get_Exchanges(database *sql.DB) ([]model.Exchange, error) {
	fmt.Println("\n-----------------------------\n API:Get_Exchanges\n-----------------------------\n")

	// return db.Get_Exchanges(database)
	exchanges, err := db.Get_Exchanges(database)
	if err != nil {
		log.Printf("api.go:Get_Exchanges error getting exchanges: %w", err)
	}

	// for _, exchange := range exchanges {

	// 	log.Println("Exchange:\n", exchange.Name)
	// 	log.Println("Timeframes:\n", exchange.Timeframes)
	// 	log.Println("rders:\n", exchange.Orders)
	// 	log.Println("Fills:\n", exchange.Fills)
	// 	log.Println("Watchlist:\n", exchange.Watchlist)
	// 	log.Println("Available_Products:\n", exchange.AvailableProducts)
	// 	log.Println("\n---------------------------------------------------\n")
	// 	log.Println("\n---------------------------------------------------\n")
	// 	log.Println("\n---------------------------------------------------\n")

	// }
	return exchanges, nil
}

// Get Candles from the Database
func Get_Candles(product, timeframe, exchange string, database *sql.DB) ([]model.Candle, error) {
	fmt.Println("\n-----------------------------\n Get_Candles:API \n-----------------------------\n")
	fmt.Println("Request:API: ", product, timeframe, exchange)

	candles, err := db.Get_Candles(product, timeframe, exchange, database)

	if err != nil {
		log.Printf("Error connecting: %v", err)
	}

	log.Print("API get candles: ", len(candles))

	return candles, nil

}
func Fetch_And_Store_Candles(exchange model.Exchange, database *sql.DB, full bool) error {
	fmt.Println("Fetch And Store Candles", exchange.Name)
	fmt.Println(len(exchange.Watchlist))
	fmt.Println(len(exchange.Timeframes))

	watchlist := exchange.Watchlist
	timeframes := exchange.Timeframes

	for _, product := range watchlist {
		for _, timeframe := range timeframes {
			end := time.Now()
			var start time.Time

			if full {
				start = end.Add(-24 * 30 * time.Hour)
			} else {
				start = end.Add(-time.Duration(exchange.CandleLimit*timeframe.Minutes) * time.Minute)
			}

			fmt.Println("\n---------------------\n", exchange.CandleLimit, "\n", timeframe.Minutes, "\n", start, "\n", end, "\n---------------------\n")
			candles, err := exchange.API.FetchCandles(product.ProductID, timeframe, start, end)
			if err != nil {
				log.Printf("Error fetching candles %s %s: %v", product.ProductID, timeframe.TF, err)
				continue
			}

			if len(candles) > 0 {
				err = db.Write_Candles(candles, product.ProductID, exchange.Name, timeframe.TF, database)
				if err != nil {
					log.Printf("Error writing candles for %s %s: %v", product.ProductID, timeframe.TF, err)
					continue
				}
				fmt.Printf("Wrote %d candles for %s %s\n", len(candles), product.ProductID, timeframe.TF)
			}
		}
	}
	return nil
}

func Do_AvailableProducts(exchange model.Exchange, database *sql.DB) error {
	available_products, err := Fetch_Available_Products(exchange)
	if err != nil {
		log.Printf("Error fetching available products for exchange: %s\n%w\n", exchange, err)
		return err
	}

	// write products to table
	if len(available_products) > 0 {
		err = db.Write_AvailableProducts(exchange.ID, available_products, database)
	}
	return nil
}

func Fetch_Available_Products(exchange model.Exchange) ([]model.Product, error) {

	available_products, err := exchange.API.FetchAvailableProducts()
	if err != nil {
		log.Printf("Error getting available products from coinbase: %w", err)
		return nil, err
	}
	return available_products, nil
}

// ORDERS
func Do_Orders_and_Fills(exchange model.Exchange, database *sql.DB) error {
	orders, err := Fetch_Orders_and_Fills(exchange)
	if err != nil {
		log.Printf("Error getting orders from exchange: %s: %v", exchange.Name, err)
		return err
	}

	log.Printf("Orders: %d", len(orders))
	if len(orders) > 0 {
		var open_orders []model.Order
		var fills []model.Fill

		for _, order := range orders {
			switch order.Status {
			case "FILLED", "SETTLED":
				// Convert the order to a fill using the conversion function
				fill := convertOrderToFill(order)
				fills = append(fills, fill)
			case "OPEN", "PENDING":
				open_orders = append(open_orders, order)
			}
		}

		log.Println("Open Orders:", len(open_orders))
		log.Println("Filled Orders:", len(fills))

		// Write open orders if there are any
		if len(open_orders) > 0 {
			err = db.Write_Orders(exchange.ID, open_orders, database)
			if err != nil {
				log.Printf("Error writing orders to db: %v", err)
				return err
			}
			log.Printf("Wrote %d open orders to database", len(open_orders))
		}

		// Write fills if there are any
		if len(fills) > 0 {
			err = db.Write_Fills(exchange.ID, fills, database)
			if err != nil {
				log.Printf("Error writing fills to db: %v", err)
				return err
			}
			log.Printf("Wrote %d fills to database", len(fills))
		}
	}

	log.Printf("Orders processing completed for %s", exchange.Name)
	return nil
}

// Helper function to convert Order to Fill
func convertOrderToFill(order model.Order) model.Fill {
	return model.Fill{
		Timestamp:      order.Timestamp,
		EntryID:        fmt.Sprintf("%s-%d", order.OrderID, order.Timestamp),
		TradeID:        order.OrderID,
		OrderID:        order.OrderID,
		TradeType:      order.TradeType,
		Price:          order.Price,
		Size:           order.Size,
		Side:           order.Side,
		Commission:     order.TotalFees,
		ProductID:      order.ProductID,
		XchID:          order.XchID,
		MarketCategory: order.MarketCategory,
	}
}

// Helper function to convert string to sql.NullString
func toNullString(s string) sql.NullString {
	return sql.NullString{
		String: s,
		Valid:  s != "",
	}
}

func Fetch_Orders_and_Fills(exchange model.Exchange) ([]model.Order, error) {
	orders, err := exchange.API.FetchOrdersFills()
	if err != nil {
		log.Printf("Error getting orders from Exchange API: %s \n%w", exchange.Name, err)
		return nil, err
	}
	log.Printf("Orders Fetched: %s", exchange.Name)
	return orders, nil
}

// func All_Candles_Loop(productID string, timeframe model.Timeframe, startTime time.Time, endTime time.Time, allCandles []model.Candle) ([]model.Candle, error) {
// 	minutes := timeframe.Minutes
// 	fmt.Println("Looping Through All Candles\n", productID, "\n", timeframe.Endpoint, "\n", minutes, "\n", startTime, "\n", endTime, "\n\n---------------")

// 	// Base case to stop recursion
// 	if startTime.After(endTime) || startTime.Equal(endTime) {
// 		fmt.Println("BREAK")
// 		return allCandles, nil // Return accumulated candles
// 	}

// 	// Retrieve candles for the current time frame
// 	candles, err := fetch_Coinbase_Candles(productID, timeframe, startTime, endTime)
// 	if err != nil {
// 		return nil, fmt.Errorf("error getting candles: %v", err)
// 	}

// 	//fmt.Println("CANDLES:\n", len(candles))
// 	allCandles = append(allCandles, candles...)
// 	//fmt.Println("Candles Total\n", len(allCandles))
// 	if len(candles) == 0 {
// 		/*
// 		   fmt.Println("\n--------------------\n")
// 		   fmt.Println("Loop Finished with: ", len(allCandles), "candles")
// 		   fmt.Println("\n--------------------\n")
// 		*/
// 		return allCandles, nil
// 	}

// 	// Move the start time backward by 300 candles worth of time
// 	newStartTime := startTime.Add(-time.Duration(350*minutes) * time.Minute)
// 	endTime = startTime

// 	// Recursive call
// 	return All_Candles_Loop(productID, timeframe, newStartTime, endTime, allCandles)
// }
// ------------------------------------------------------------------------

// ------------------------------------------------------------------------
//
// ------------------------------------------------------------------------

// func Gap_Search(exchange model.Exchange) {
// 	fmt.Println("Data gap search")

// 	// for each asset in watchlist
// 	//		for each timeframe in timeframe
// 	//			Get All candles from db
// 	//
// 	//			interval := time.Now().Add(-time.Duration(300 * timeframe.Minutes) * time.Minute)
// 	//			for each candle := candles:
// 	//				candle_a = candle[

// }

// ------------------------------------------------------------------------
//
// ------------------------------------------------------------------------

// ------------------------------------------------------------------------

// func Check_Candle_Gaps(exchange model.Exchange) {
// 	fmt.Println("\n------------------\nCheck Candle Gaps")
// 	//fmt.Println(exchange)

// 	name := exchange.Name
// 	watchlist := exchange.Watchlist
// 	timeframes := exchange.Timeframes

// 	fmt.Println("Exchange: ", name)
// 	fmt.Println("Watchlist: ", watchlist)
// 	fmt.Println("Timeframes: ", timeframes)
// 	fmt.Println("\n------------------\n")

// 	//var gaps = []string{}
// 	for asset, _ := range watchlist {
// 		fmt.Println("ASSET", watchlist[asset].Product)
// 		for tf, _ := range timeframes {
// 			product := watchlist[asset].Product
// 			fmt.Printf("%s_%s_%s\n", name, watchlist[asset].Product, timeframes[tf].TF)
// 			candles, err := db.Get_All_Candles(watchlist[asset].Product, timeframes[tf].TF, name)
// 			if err != nil {
// 				fmt.Printf("Error scanning candles: %v", err)
// 			}

// 			count := 0
// 			for i := 1; i < len(candles)-1; i++ {
// 				timeframe := timeframes[tf]
// 				c_1 := candles[i].Timestamp
// 				c_1_time := time.Unix(c_1, 0)
// 				c_0 := candles[i-1].Timestamp
// 				c_0_time := time.Unix(c_0, 0)
// 				delta := c_1 - c_0
// 				//fmt.Println("---------------------------------\n", c_0, c_0_time, "\n", c_1, c_1_time)
// 				if delta > timeframe.Minutes*60 {
// 					count++
// 					num_candles := delta / (timeframe.Minutes * 60)
// 					fmt.Println("\n------------------------------\nGAP Found: ")
// 					fmt.Println(delta, timeframe.Minutes*60)
// 					fmt.Println(product, timeframe.TF)
// 					fmt.Printf("%s\n%s\n---------------------\n", c_0_time, c_1_time)
// 					fmt.Println("Gap:", count)
// 					fmt.Println(delta/60, "minutes")
// 					fmt.Println(num_candles, "Candles")

// 					all_candles, err := Get_Coinbase_Candles(product, timeframe, c_0_time, c_1_time)
// 					if err != nil {
// 						fmt.Println("Error getting candles: ", err, product, timeframe.TF)
// 					}
// 					err = db.Write_Candles(all_candles, product, exchange.Name, timeframe.TF)
// 					if err != nil {
// 						fmt.Println("Error Writing candles: ", product, timeframe.TF, err)
// 					}
// 				}
// 			}
// 		}
// 	}
// 	/*
// 		for asset, _ := range exchange.Watchlist {
// 			for tf, _ := range exchange.Timeframes {
// 				fmt.Println(exchange.Name, exchange.Watchlist[asset], exchange.Timeframes[tf])
// 				candles, err := db.Get_All_Candles(exchange.Name, exchange.Wathclist[asset], exchange.Timeframes[tf]) if err != nil {
// 					fmt.Printf("Error scanning candles: %v", err)
// 				}

// 				temp := candles[0]
// 				for i:=0; i < len(candles) - 1; i++ {
// 					temp = candles[i]
// 					temp2 := candles[i+1]

// 					time1 := temp.Time
// 					time2 := temp2.Time

// 					fmt.Println("Time Delta", time2, time1)
// 					fmt.Println(exchange.Timeframes.Minutes)
// 				}
// 			}
// 		}
// 	*/
// }

//func Gap_Check(candles []model.Candle ) (gap
