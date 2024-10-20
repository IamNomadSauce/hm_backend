package model

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type CoinbaseAPI struct {
	APIKey       string
	APISecret    string
	BaseURL      string
	RateLimit    int
	CandleLimit  int
	RateWindow   time.Duration
	RequestCount int
	LastRequest  time.Time

	SupportedOrderTypes []string
	SupportedTimeframes []string
	MinimumOrderSizes   map[string]float64
	MakerFee            float64
	TakerFee            float64
}

// Exchange operation
func (api *CoinbaseAPI) FetchOrders(exchange *Exchange) ([]Order, error) {
	var orders []Order
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}

	apiKey := api.APIKey
	apiSecret := api.APISecret

	timestamp := time.Now().Unix()
	baseURL := api.BaseURL
	path := "/api/v3/brokerage/orders/historical/batch"
	method := "GET"
	body := ""

	signature := GetCBSign(apiSecret, timestamp, method, path, body)

	client := &http.Client{}
	req, err := http.NewRequest(method, baseURL+path, nil)
	if err != nil {
		fmt.Println("NewRequest: ", err)
		return nil, fmt.Errorf("Error making Coinbase orders request: %w", err)
	}

	req.Header.Add("CB-ACCESS-SIGN", signature)
	req.Header.Add("CB-ACCESS-TIMESTAMP", strconv.FormatInt(timestamp, 10))
	req.Header.Add("CB-ACCESS-KEY", apiKey)
	req.Header.Add("CB-VERSION", "2015-07-22")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error DO: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Readall Err: %w", err)
	}

	var response struct {
		Orders []Order `json:"orders"`
	}

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling JSON: %w", err)
	}

	var filteredOrders []Order
	for _, order := range response.Orders {
		if order.Status != "CANCELLED" {
			filteredOrders = append(filteredOrders, order)
		}
	}

	return orders, nil
}

// Exchange operation
func (api *CoinbaseAPI) FetchCandles(productID string, timeframe Timeframe, start, end time.Time) ([]Candle, error) {
	var candles []Candle
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
	currentStart := start
	for currentStart.Before(end) {
		currentEnd := currentStart.Add(time.Duration(maxCandles) * candleDuration)
		if currentEnd.After(end) {
			currentEnd = end
		}

		res, err := fetch_Coinbase_Candles(productID, timeframe, currentStart, currentEnd)
		if err != nil {
			return nil, err
		}

		candles = append(candles, res...)
		currentStart = currentEnd
	}
	return candles, nil
}

func fetch_Coinbase_Candles(productID string, timeframe Timeframe, start, end time.Time) ([]Candle, error) {
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
		Candles []Candle `json:"candles"`
	}

	err = json.Unmarshal(body, &candleData)
	if err != nil {
		fmt.Println("Error unmarshalling into candleData")
		return nil, fmt.Errorf("Error decoding JSON: %v", err)
	}
	fmt.Println("Candles: \n", len(candleData.Candles))
	return candleData.Candles, nil
}

// Exchange operation
func (api *CoinbaseAPI) FetchFills() ([]Fill, error) {
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
		return nil, fmt.Errorf("Error unmarshalling JSON: %w", err)
	}

	fills = response.Fills

	return fills, nil
}

// Exchange operation
func (api *CoinbaseAPI) FetchPortfolio() ([]Asset, error) {
	var accounts []Asset
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
		return nil, fmt.Errorf("Error Coinbase Portfolio New Request: %w", err)
	}

	req.Header.Add("CB-ACCESS-SIGN", signature)
	req.Header.Add("CB-ACCESS-TIMESTAMP", strconv.FormatInt(timestamp, 10))
	req.Header.Add("CB-ACCESS-KEY", apiKey)
	req.Header.Add("CB-VERSION", "2015-07-22")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Fetch coinbase portfolio Error DO: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Fetch coinbase portfolio Readall Err: %w", err)
	}

	var response struct {
		Accounts []Asset `json:"accounts"`
	}

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshaling JSON: %w", err)
	}

	total := 0.0
	for _, acct := range response.Accounts {
		balance, _ := strconv.ParseFloat(acct.AvailableBalance.Value, 64)
		hold_balance, _ := strconv.ParseFloat(acct.Hold.Value, 64)
		if balance > 0 {
			price, _ := GetPrice(acct.Symbol.Name)
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

	return accounts, nil
}

func GetCBSign(apiSecret string, timestamp int64, method, path, body string) string {
	message := fmt.Sprintf("%d%s%s%s", timestamp, method, path, body)
	hasher := hmac.New(sha256.New, []byte(apiSecret))
	hasher.Write([]byte(message))
	signature := hex.EncodeToString(hasher.Sum(nil))
	return signature
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
