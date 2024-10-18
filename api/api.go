package api

import (
	"backend/db"
	"backend/model"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	_ "database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

func ApiConnect() {
	fmt.Println("API Connected")
}

type Fill struct {
	EntryID             string      `json:"entry_id"`
	TradeID             string      `json:"trade_id"`
	OrderID             string      `json:"order_id"`
	TradeTime           string      `json:"trade_time"`
	TradeType           string      `json:"trade_type"`
	Price               string      `json:"price"`
	Size                string      `json:"size"`
	Commission          string      `json:"commission"`
	ProductID           string      `json:"product_id"`
	SequenceTimestamp   string      `json:"sequence_timestamp"`
	LiquidityIndicator  string      `json:"liquidity_indicator"`
	SizeInQuote         interface{} `json:"size_in_quote"`
	UserID              string      `json:"user_id"`
	Side                string      `json:"side"`
	OrderType           string      `json:"order_type"`
	ClientOrderID       string      `json:"client_order_id,omitempty"`
	Fee                 string      `json:"fee"`
	FeeRate             string      `json:"fee_rate"`
	TotalFees           string      `json:"total_fees"`
	TotalValueAfterFees string      `json:"total_value_after_fees"`
	TotalValueInQuote   string      `json:"total_value_in_quote"`
}

func GetCBSign(apiSecret string, timestamp int64, method, path, body string) string {
	message := fmt.Sprintf("%d%s%s%s", timestamp, method, path, body)
	hasher := hmac.New(sha256.New, []byte(apiSecret))
	hasher.Write([]byte(message))
	signature := hex.EncodeToString(hasher.Sum(nil))
	return signature
}

func Get_Coinbase_Fills() []Fill {
	var fills []Fill
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}

	apiKey := os.Getenv("CBAPIKEY")
	apiSecret := os.Getenv("CBAPISECRET")

	timestamp := time.Now().Unix()
	baseURL := "https://api.coinbase.com"
	path := "/api/v3/brokerage/orders/historical/fills"
	method := "GET"
	body := ""

	signature := GetCBSign(apiSecret, timestamp, method, path, body)

	client := &http.Client{}
	req, err := http.NewRequest(method, baseURL+path, nil)
	if err != nil {
		fmt.Println("NewRequest: ", err)
	}

	req.Header.Add("CB-ACCESS-SIGN", signature)
	req.Header.Add("CB-ACCESS-TIMESTAMP", strconv.FormatInt(timestamp, 10))
	req.Header.Add("CB-ACCESS-KEY", apiKey)
	req.Header.Add("CB-VERSION", "2015-07-22")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error DO: ", err)
	}

	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Readall Err ", err)
	}

	var response struct {
		Fills []Fill `json:"fills"`
	}

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		fmt.Println("Error unmarshalling JSON: ", err)
		return fills
	}

	fills = response.Fills

	return fills
}

// --------------------------------------------------------------------------
// Orders
// --------------------------------------------------------------------------

type Order struct {
	OrderID              string      `json:"order_id"`
	ClientOrderID        string      `json:"client_order_id"`
	ProductID            string      `json:"product_id"`
	UserID               string      `json:"user_id"`
	OrderConfiguration   interface{} `json:"order_configuration"`
	Side                 string      `json:"side"`
	Status               string      `json:"status"`
	TimeInForce          string      `json:"time_in_force"`
	CreatedTime          string      `json:"created_time"`
	CompletionPercentage string      `json:"completion_percentage"`
	FilledSize           string      `json:"filled_size"`
	AverageFilledPrice   string      `json:"average_filled_price"`
	Fee                  string      `json:"fee"`
	NumberOfFills        string      `json:"number_of_fills"`
	FilledValue          string      `json:"filled_value"`
	PendingCancel        bool        `json:"pending_cancel"`
	SizeInQuote          bool        `json:"size_in_quote"`
	TotalFees            string      `json:"total_fees"`
	SizeInclusiveOfFees  interface{} `json:"size_inclusive_of_fees"`
	TotalValueAfterFees  string      `json:"total_value_after_fees"`
	TriggerStatus        string      `json:"trigger_status"`
	OrderType            string      `json:"order_type"`
	RejectReason         string      `json:"reject_reason"`
	Settled              bool        `json:"settled"`
	ProductType          string      `json:"product_type"`
	RejectMessage        string      `json:"reject_message"`
	CancelMessage        string      `json:"cancel_message"`
}

func Get_Coinbase_Orders() []Order {
	var orders []Order
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}

	apiKey := os.Getenv("CBAPIKEY")
	apiSecret := os.Getenv("CBAPISECRET")

	timestamp := time.Now().Unix()
	baseURL := "https://api.coinbase.com"
	path := "/api/v3/brokerage/orders/historical/batch"
	method := "GET"
	body := ""

	signature := GetCBSign(apiSecret, timestamp, method, path, body)

	client := &http.Client{}
	req, err := http.NewRequest(method, baseURL+path, nil)
	if err != nil {
		fmt.Println("NewRequest: ", err)
		return orders
	}

	req.Header.Add("CB-ACCESS-SIGN", signature)
	req.Header.Add("CB-ACCESS-TIMESTAMP", strconv.FormatInt(timestamp, 10))
	req.Header.Add("CB-ACCESS-KEY", apiKey)
	req.Header.Add("CB-VERSION", "2015-07-22")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error DO: ", err)
		return orders
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Readall Err ", err)
		return orders
	}

	var response struct {
		Orders []Order `json:"orders"`
	}

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		fmt.Println("Error unmarshaling JSON: ", err)
		return orders
	}

	var filteredOrders []Order
	for _, order := range response.Orders {
		if order.Status != "CANCELLED" {
			filteredOrders = append(filteredOrders, order)
		}
	}

	return filteredOrders
}

// --------------------------------------------------------------------------
// Account
// --------------------------------------------------------------------------

type Account struct {
	UUID             string  `json:"uuid"`
	Name             string  `json:"name"`
	Currency         string  `json:"currency"`
	AvailableBalance Balance `json:"available_balance"`
	Default          bool    `json:"default"`
	Active           bool    `json:"active"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
	DeletedAt        string  `json:"deleted_at"`
	Type             string  `json:"type"`
	Ready            bool    `json:"ready"`
	Hold             Balance `json:"hold"`
}

type Balance struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

func Get_Coinbase_Account_Balance() []Account {
	var accounts []Account
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}

	apiKey := os.Getenv("CBAPIKEY")
	apiSecret := os.Getenv("CBAPISECRET")

	timestamp := time.Now().Unix()
	baseURL := "https://api.coinbase.com"
	path := "/api/v3/brokerage/accounts"
	method := "GET"
	body := ""

	signature := GetCBSign(apiSecret, timestamp, method, path, body)

	client := &http.Client{}
	req, err := http.NewRequest(method, baseURL+path, nil)
	if err != nil {
		fmt.Println("NewRequest: ", err)
		return accounts
	}

	req.Header.Add("CB-ACCESS-SIGN", signature)
	req.Header.Add("CB-ACCESS-TIMESTAMP", strconv.FormatInt(timestamp, 10))
	req.Header.Add("CB-ACCESS-KEY", apiKey)
	req.Header.Add("CB-VERSION", "2015-07-22")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error DO: ", err)
		return accounts
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Readall Err ", err)
		return accounts
	}

	var response struct {
		Accounts []Account `json:"accounts"`
	}

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		fmt.Println("Error unmarshaling JSON: ", err)
		return accounts
	}

	total := 0.0
	for _, acct := range response.Accounts {
		balance, _ := strconv.ParseFloat(acct.AvailableBalance.Value, 64)
		hold_balance, _ := strconv.ParseFloat(acct.Hold.Value, 64)
		if balance > 0 {
			price, _ := GetPrice(acct.Currency)
			value := price * balance
			hold_value := price * hold_balance
			total += value
			total += hold_value
			//fmt.Println(acct.Currency, price, balance, value)

			accounts = append(accounts, acct)
		}

	}
	fmt.Println(total)

	fmt.Println(len(accounts))

	return accounts
}

type PriceInfo struct {
	Currency string
	Price    float64
	Value    float64
}

func GetPrice(currency string) (float64, error) {
	if currency == "USD" || currency == "USDT" || currency == "USDC" {
		return 1.0, nil
	}

	apiKey := os.Getenv("CBAPIKEY")
	apiSecret := os.Getenv("CBAPISECRET")

	timestamp := time.Now().Unix()
	baseURL := "https://api.coinbase.com"
	path := fmt.Sprintf("/api/v3/brokerage/products/%s-USD", currency)
	method := "GET"
	body := ""

	signature := GetCBSign(apiSecret, timestamp, method, path, body)

	client := &http.Client{}
	req, err := http.NewRequest(method, baseURL+path, nil)
	if err != nil {
		return 0, fmt.Errorf("NewRequest: %v", err)
	}

	req.Header.Add("CB-ACCESS-SIGN", signature)
	req.Header.Add("CB-ACCESS-TIMESTAMP", strconv.FormatInt(timestamp, 10))
	req.Header.Add("CB-ACCESS-KEY", apiKey)
	req.Header.Add("CB-VERSION", "2015-07-22")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("Error DO: %v", err)
	}
	defer resp.Body.Close()

	var tickerResponse struct {
		Price string `json:"price"`
	}

	err = json.NewDecoder(resp.Body).Decode(&tickerResponse)
	if err != nil {
		return 0, fmt.Errorf("Error decoding JSON: %v", err)
	}

	price, err := strconv.ParseFloat(tickerResponse.Price, 64)
	if err != nil {
		return 0, fmt.Errorf("Error parsing price: %v", err)
	}

	return price, nil
}

// --------------------------------------------------------------------------
// Candles
// --------------------------------------------------------------------------

func Get_Coinbase_Candles(productID string, timeframe model.Timeframe, start, end time.Time) ([]model.Candle, error) {
	fmt.Println("\n-------------------------\nGet_Coinbase_Candles \n", productID, "\n", timeframe.Endpoint, "\n", start, "\n", end, "\n")

	// Maximum number of candles per request
	const maxCandles = 350

	// Calculate the duration of one candle
	candleDuration := time.Duration(timeframe.Minutes) * time.Minute

	// Calculate the total duration
	totalDuration := end.Sub(start)

	// Calculate the number of candles in the total duration
	totalCandles := int(totalDuration / candleDuration)

	// If the total number of candles is less than or equal to maxCandles, make a single request
	if totalCandles <= maxCandles {
		return fetch_Coinbase_Candles(productID, timeframe, start, end)
	}

	// Otherwise, split the request into multiple calls
	var allCandles []model.Candle
	currentStart := start
	for currentStart.Before(end) {
		currentEnd := currentStart.Add(time.Duration(maxCandles) * candleDuration)
		if currentEnd.After(end) {
			currentEnd = end
		}

		candles, err := fetch_Coinbase_Candles(productID, timeframe, currentStart, currentEnd)
		if err != nil {
			return nil, err
		}

		allCandles = append(allCandles, candles...)
		currentStart = currentEnd
	}
	return allCandles, nil
}

func fetch_Coinbase_Candles(productID string, timeframe model.Timeframe, start, end time.Time) ([]model.Candle, error) {
	fmt.Println("\n-------------------------\nGet_Coinbase_Candles \n", productID, "\n", timeframe.Endpoint, "\n", start, "\n", end, "\n")
	apiKey := os.Getenv("CBAPIKEY")
	apiSecret := os.Getenv("CBAPISECRET")

	baseURL := "https://api.coinbase.com"
	path := fmt.Sprintf("/api/v3/brokerage/products/%s/candles", productID)
	method := "GET"

	query := url.Values{}
	query.Add("granularity", timeframe.Endpoint)
	query.Add("start", strconv.FormatInt(start.Unix(), 10))
	query.Add("end", strconv.FormatInt(end.Unix(), 10))

	fullURL := fmt.Sprintf("%s%s?%s", baseURL, path, query.Encode())

	timestamp := time.Now().Unix()
	signature := GetCBSign(apiSecret, timestamp, "GET", path, "")

	client := &http.Client{}
	req, err := http.NewRequest(method, fullURL, nil)
	if err != nil {
		fmt.Println("Error with new Request")
		return nil, fmt.Errorf("NewRequest: %v", err)
	}

	req.Header.Add("CB-ACCESS-SIGN", signature)
	req.Header.Add("CB-ACCESS-TIMESTAMP", strconv.FormatInt(timestamp, 10))
	req.Header.Add("CB-ACCESS-KEY", apiKey)
	req.Header.Add("CB-VERSION", "2015-07-22")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error DO: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Println("Error fetching candles")
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("Error fetching candles: %d - %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body")
		return nil, fmt.Errorf("Error reading response body: %v", err)
	}

	var candleData struct {
		Candles []model.Candle `json:"candles"`
	}

	err = json.Unmarshal(body, &candleData)
	if err != nil {
		fmt.Println("Error unmarshalling into candleData")
		return nil, fmt.Errorf("Error decoding JSON: %v", err)
	}
	fmt.Println("Candles: \n", len(candleData.Candles))
	return candleData.Candles, nil
}

func All_Candles_Loop(productID string, timeframe model.Timeframe, startTime time.Time, endTime time.Time, allCandles []model.Candle) ([]model.Candle, error) {
	minutes := timeframe.Minutes
	fmt.Println("Looping Through All Candles\n", productID, "\n", timeframe.Endpoint, "\n", minutes, "\n", startTime, "\n", endTime, "\n\n---------------")

	// Base case to stop recursion
	if startTime.After(endTime) || startTime.Equal(endTime) {
		fmt.Println("BREAK")
		return allCandles, nil // Return accumulated candles
	}

	// Retrieve candles for the current time frame
	candles, err := fetch_Coinbase_Candles(productID, timeframe, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("error getting candles: %v", err)
	}

	//fmt.Println("CANDLES:\n", len(candles))
	allCandles = append(allCandles, candles...)
	//fmt.Println("Candles Total\n", len(allCandles))
	if len(candles) == 0 {
		/*
		   fmt.Println("\n--------------------\n")
		   fmt.Println("Loop Finished with: ", len(allCandles), "candles")
		   fmt.Println("\n--------------------\n")
		*/
		return allCandles, nil
	}

	// Move the start time backward by 300 candles worth of time
	newStartTime := startTime.Add(-time.Duration(350*minutes) * time.Minute)
	endTime = startTime

	// Recursive call
	return All_Candles_Loop(productID, timeframe, newStartTime, endTime, allCandles)
}

func Fetch_And_Store_Candles(exchange model.Exchange, database *sql.DB) error {
	fmt.Println("Fill Whole Exchange")
	fmt.Println(exchange.Watchlist)
	fmt.Println(exchange.Timeframes)

	watchlist := exchange.Watchlist
	timeframes := exchange.Timeframes

	for _, product := range exchange.Watchlist {
		for _, timeframe := range exchange.Timeframes {
			end := time.Now()
			start := end.Add(-time.Duration(exchange.API. *timeframe.Minutes) * time.Minute)
			candles, err := exchange.API.FetchCandles(product.Name, timeframe.TF, start, end)
			if err != nil {
				return fmt.Errorf("Error fetching candles for %s %s: %w", product.Name, timeframe.TF, err)
			}
			err = db.Write_Candles(candles, product.Name, exchange.Name, timeframe.TF, database)
			if err != nil {
				fmt.Println("Error Writing candles: ", product, timeframe.TF, err)
			}
		}
		return nil
	}

	for wl, _ := range watchlist {

		product := watchlist[wl].Product

		for tf, _ := range timeframes {
			timeframe := timeframes[tf]
			fmt.Println(product)
			var candles []model.Candle
			start_time := time.Now().Add(-time.Duration(350*timeframe.Minutes) * time.Minute)
			if full {
				candles, err := exchange.API.FetchCandles(product.Name, timeframe.TF, start_time, end)
				candles, err := All_Candles_Loop(product, timeframe, start_time, time.Now(), candles)
				if err != nil {
					fmt.Println("Error getting all candles", err, product, timeframe.TF)
				}
				err = db.Write_Candles(candles, product, exchange.Name, timeframe.TF)
				if err != nil {
					fmt.Println("Error Writing candles: ", product, timeframe.TF, err)
				}

			} else {
				minutes := timeframes[tf].Minutes
				end := time.Now()
				start := end.Add(-time.Duration(minutes*350) * time.Minute)
				candles, err := fetch_Coinbase_Candles(product, timeframe, start, end)
				if err != nil {
					fmt.Println("Error getting candles: ", err, product, timeframe.TF)
				}
				err = db.Write_Candles(candles, product, exchange.Name, timeframe.TF)
				if err != nil {
					fmt.Println("Error Writing candles: ", product, timeframe.TF, err)
				}
			}
		}
	}
	return nil
}

// ------------------------------------------------------------------------
//
// ------------------------------------------------------------------------

func Gap_Search(exchange model.Exchange) {
	fmt.Println("Data gap search")

	// for each asset in watchlist
	//		for each timeframe in timeframe
	//			Get All candles from db
	//
	//			interval := time.Now().Add(-time.Duration(300 * timeframe.Minutes) * time.Minute)
	//			for each candle := candles:
	//				candle_a = candle[

}

// ------------------------------------------------------------------------
//
// ------------------------------------------------------------------------

func Get_Exchanges() ([]model.Exchange, error) {
	fmt.Println("\n-----------------------------\n API:Get_Exchanges\n-----------------------------\n")

	return db.Get_Exchanges()
}

// Get Candles from the Database
func Get_Candles(product, timeframe, exchange string) ([]model.Candle, error) {
	fmt.Println("\n-----------------------------\n Get_Candles:API \n-----------------------------\n")
	fmt.Println("Request:API: ", product, timeframe, exchange)

	candles, err := db.Get_Candles(product, timeframe, exchange)

	if err != nil {
		log.Printf("Error connecting: %v", err)
	}

	log.Print("API get candles: ", len(candles))

	return candles, nil

}

// ------------------------------------------------------------------------

func Check_Candle_Gaps(exchange model.Exchange) {
	fmt.Println("\n------------------\nCheck Candle Gaps")
	//fmt.Println(exchange)

	name := exchange.Name
	watchlist := exchange.Watchlist
	timeframes := exchange.Timeframes

	fmt.Println("Exchange: ", name)
	fmt.Println("Watchlist: ", watchlist)
	fmt.Println("Timeframes: ", timeframes)
	fmt.Println("\n------------------\n")

	//var gaps = []string{}
	for asset, _ := range watchlist {
		fmt.Println("ASSET", watchlist[asset].Product)
		for tf, _ := range timeframes {
			product := watchlist[asset].Product
			fmt.Printf("%s_%s_%s\n", name, watchlist[asset].Product, timeframes[tf].TF)
			candles, err := db.Get_All_Candles(watchlist[asset].Product, timeframes[tf].TF, name)
			if err != nil {
				fmt.Printf("Error scanning candles: %v", err)
			}

			count := 0
			for i := 1; i < len(candles)-1; i++ {
				timeframe := timeframes[tf]
				c_1 := candles[i].Timestamp
				c_1_time := time.Unix(c_1, 0)
				c_0 := candles[i-1].Timestamp
				c_0_time := time.Unix(c_0, 0)
				delta := c_1 - c_0
				//fmt.Println("---------------------------------\n", c_0, c_0_time, "\n", c_1, c_1_time)
				if delta > timeframe.Minutes*60 {
					count++
					num_candles := delta / (timeframe.Minutes * 60)
					fmt.Println("\n------------------------------\nGAP Found: ")
					fmt.Println(delta, timeframe.Minutes*60)
					fmt.Println(product, timeframe.TF)
					fmt.Printf("%s\n%s\n---------------------\n", c_0_time, c_1_time)
					fmt.Println("Gap:", count)
					fmt.Println(delta/60, "minutes")
					fmt.Println(num_candles, "Candles")

					all_candles, err := Get_Coinbase_Candles(product, timeframe, c_0_time, c_1_time)
					if err != nil {
						fmt.Println("Error getting candles: ", err, product, timeframe.TF)
					}
					err = db.Write_Candles(all_candles, product, exchange.Name, timeframe.TF)
					if err != nil {
						fmt.Println("Error Writing candles: ", product, timeframe.TF, err)
					}
				}
			}
		}
	}
	/*
		for asset, _ := range exchange.Watchlist {
			for tf, _ := range exchange.Timeframes {
				fmt.Println(exchange.Name, exchange.Watchlist[asset], exchange.Timeframes[tf])
				candles, err := db.Get_All_Candles(exchange.Name, exchange.Wathclist[asset], exchange.Timeframes[tf]) if err != nil {
					fmt.Printf("Error scanning candles: %v", err)
				}

				temp := candles[0]
				for i:=0; i < len(candles) - 1; i++ {
					temp = candles[i]
					temp2 := candles[i+1]

					time1 := temp.Time
					time2 := temp2.Time

					fmt.Println("Time Delta", time2, time1)
					fmt.Println(exchange.Timeframes.Minutes)
				}
			}
		}
	*/
}

//func Gap_Check(candles []model.Candle ) (gap
