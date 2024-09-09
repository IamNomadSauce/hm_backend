package api

import (
	"backend/model"
	"backend/db"
	_"database/sql"
	"net/url"
	"fmt"
	"os"
	"github.com/joho/godotenv"
	"encoding/json"
	"time"
	"strconv"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"io/ioutil"
	
)

func ApiConnect() {
	fmt.Println("API Connected")
}
type Fill struct {
    EntryID            string      `json:"entry_id"`
    TradeID            string      `json:"trade_id"`
    OrderID            string      `json:"order_id"`
    TradeTime          string      `json:"trade_time"`
    TradeType          string      `json:"trade_type"`
    Price              string      `json:"price"`
    Size               string      `json:"size"`
    Commission         string      `json:"commission"`
    ProductID          string      `json:"product_id"`
    SequenceTimestamp  string      `json:"sequence_timestamp"`
    LiquidityIndicator string      `json:"liquidity_indicator"`
    SizeInQuote        interface{} `json:"size_in_quote"`
    UserID             string      `json:"user_id"`
    Side               string      `json:"side"`
    OrderType          string      `json:"order_type"`
    ClientOrderID      string      `json:"client_order_id,omitempty"`
    Fee                string      `json:"fee"`
    FeeRate            string      `json:"fee_rate"`
    TotalFees          string      `json:"total_fees"`
    TotalValueAfterFees string     `json:"total_value_after_fees"`
    TotalValueInQuote  string      `json:"total_value_in_quote"`
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
    UUID        string `json:"uuid"`
    Name        string `json:"name"`
    Currency    string `json:"currency"`
    AvailableBalance Balance `json:"available_balance"`
    Default     bool   `json:"default"`
    Active      bool   `json:"active"`
    CreatedAt   string `json:"created_at"`
    UpdatedAt   string `json:"updated_at"`
    DeletedAt   string `json:"deleted_at"`
    Type        string `json:"type"`
    Ready       bool   `json:"ready"`
    Hold        Balance `json:"hold"`
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

func Get_Coinbase_Candles(productID string, granularity string, start, end time.Time) ([]model.Candle, error) {
	fmt.Println("Get_Coinbase_Candles \n",productID, "\n", granularity, "\n", start, "\n", end, "\n")
    apiKey := os.Getenv("CBAPIKEY")
    apiSecret := os.Getenv("CBAPISECRET")

    baseURL := "https://api.coinbase.com"
    path := fmt.Sprintf("/api/v3/brokerage/products/%s/candles", productID)
    method := "GET"

    query := url.Values{}
    query.Add("granularity", granularity)
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
	//fmt.Println("Candles: \n", candleData.Candles)
    return candleData.Candles, nil
}

func All_Candles_Loop(productID string, granularity string, minutes int, startTime time.Time, endTime time.Time, allCandles []model.Candle) ([]model.Candle, error) {
    fmt.Println("Looping Through All Candles\n", productID, "\n", granularity, "\n", minutes, "\n", startTime, "\n", endTime, "\n\n---------------")

    // Base case to stop recursion
    if startTime.After(endTime) || startTime.Equal(endTime) {
        fmt.Println("BREAK")
        return allCandles, nil  // Return accumulated candles
    }

    // Retrieve candles for the current time frame
    candles, err := Get_Coinbase_Candles(productID, granularity, startTime, endTime)
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
    newStartTime := startTime.Add(-time.Duration(300 * minutes) * time.Minute)
    endTime = startTime

    // Recursive call
    return All_Candles_Loop(productID, granularity, minutes, newStartTime, endTime, allCandles)
}

func Fill_Exchange(exchange model.Exchange, full bool) error{
	fmt.Println("Fill Whole Exchange")
	fmt.Println(exchange.Watchlist)
	fmt.Println(exchange.Timeframes)


	watchlist := exchange.Watchlist
	timeframes := exchange.Timeframes

	for wl, _ := range watchlist {

		product := watchlist[wl].Product
		
		for tf, _ := range timeframes {
			timeframe := timeframes[tf]
			fmt.Println(product)
			var candles []model.Candle
			start_time := time.Now().Add(-time.Duration(300 * timeframe.Minutes) * time.Minute)
			if full {
				candles, err := All_Candles_Loop(product, timeframe.Endpoint, timeframe.Minutes, start_time, time.Now(), candles)
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
				start := end.Add(-time.Duration(minutes * 300) * time.Minute)
				candles, err := Get_Coinbase_Candles(product, timeframe.Endpoint, start, end)
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

func Get_Candles(exchange, product, timeframe string) []model.Candle {
	fmt.Println("Get_Candles:API")

	candles, err := db.Get_Candles()


}














