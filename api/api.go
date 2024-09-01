package api

import (
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

	for _, acct := range response.Accounts {
		balance, _ := strconv.ParseFloat(acct.AvailableBalance.Value, 64) 
		if balance > 0 {
			accounts = append(accounts, acct)
		}

	}

	fmt.Println(len(accounts))

    return accounts
}



