package model

import (
	"backend/model"
	"bytes"
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

type AlpacaAPI struct {
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

func (api *AlpacaAPI) PlaceBracketOrder(trade_group model.TradeGroup) error {
	return nil
}

func (api *AlpacaAPI) PlaceOrder(orderBody interface{}) (string, error) {
	timestamp := time.Now().Unix()
	path := "/api/v3/brokerage/orders" // Fixed typo in path
	method := "POST"

	bodyBytes, err := json.Marshal(orderBody)
	if err != nil {
		return "", fmt.Errorf("Error marshaling request body: %w", err)
	}

	signature := GetCBSign(api.APISecret, timestamp, method, path, string(bodyBytes))

	req, err := http.NewRequest(method, api.BaseURL+path, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("Error creating order request: %w", err)
	}

	req.Header.Add("CB-ACCESS-SIGN", signature)
	req.Header.Add("CB-ACCESS-TIMESTAMP", strconv.FormatInt(timestamp, 10))
	req.Header.Add("CB-ACCESS-KEY", api.APIKey)
	req.Header.Add("CB-VERSION", "2015-07-22")
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Error executing order request: %w", err)
	}
	defer resp.Body.Close()

	var response struct {
		OrderID string `json:"order_id"`
		Success bool   `json:"success"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("Error decoding response: %w", err)
	}

	if !response.Success {
		return "", fmt.Errorf("Order placement unsuccessful")
	}

	return response.OrderID, nil
}

func (api *AlpacaAPI) GetOrder(orderID string) (*CoinbaseOrder, error) {
	timestamp := time.Now().Unix()
	path := fmt.Sprintf("/api/v3/brokerage/orders/get_order?order_id=%s", orderID)
	method := "GET"

	signature := GetCBSign(api.APISecret, timestamp, method, path, "")

	req, err := http.NewRequest(method, api.BaseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating get order request: %w", err)
	}

	req.Header.Add("CB-ACCESS-SIGN", signature)
	req.Header.Add("CB-CB-ACCESS-TIMESTAMP", strconv.FormatInt(timestamp, 10))
	req.Header.Add("CB-ACCESS-KEY", api.APIKey)
	req.Header.Add("CB-VERSION", "2015-07-22")
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error executing get order request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("Error getting orger: status %d - %s", resp.StatusCode, string(body))
	}

	var order CoinbaseOrder
	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		return nil, fmt.Errorf("Error decoding order response: %w", err)
	}

	return &order, nil
}

func (api *AlpacaAPI) FetchAvailableProducts() ([]Product, error) {
	var products []Product
	// if api == nil {
	// 	return nil, fmt.Errorf("CoinbaseAPI is not initialized")
	// }

	// path := "/api/v3/brokerage/products"
	// method := "GET"

	// // Construct the full URL
	// fullURL := fmt.Sprintf("%s%s", api.BaseURL, path)

	// // Create timestamp for authentication
	// timestamp := time.Now().Unix()
	// signature := GetCBSign(api.APISecret, timestamp, method, path, "")

	// // Create new request
	// req, err := http.NewRequest(method, fullURL, nil)
	// if err != nil {
	// 	return nil, fmt.Errorf("error creating request: %w", err)
	// }

	// // Add headers
	// req.Header.Add("CB-ACCESS-SIGN", signature)
	// req.Header.Add("CB-ACCESS-TIMESTAMP", strconv.FormatInt(timestamp, 10))
	// req.Header.Add("CB-ACCESS-KEY", api.APIKey)
	// req.Header.Add("CB-VERSION", "2015-07-22")

	// // Make the request
	// client := &http.Client{}
	// resp, err := client.Do(req)
	// if err != nil {
	// 	return nil, fmt.Errorf("error making request: %w", err)
	// }
	// defer resp.Body.Close()

	// if resp.StatusCode != http.StatusOK {
	// 	body, _ := ioutil.ReadAll(resp.Body)
	// 	return nil, fmt.Errorf("error response from Coinbase: %d - %s", resp.StatusCode, string(body))
	// }

	// // Read and parse the response
	// var response struct {
	// 	Products []struct {
	// 		ProductID string `json:"product_id"`
	// 		BaseName  string `json:"base_name"`
	// 		QuoteName string `json:"quote_name"`
	// 		Status    string `json:"status"`
	// 	} `json:"products"`
	// }

	// if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
	// 	return nil, fmt.Errorf("error decoding response: %w", err)
	// }

	// // Convert to our Product type
	// var products []Product
	// for _, p := range response.Products {
	// 	if p.Status == "online" { // Only include active products
	// 		products = append(products, Product{
	// 			ProductID: p.ProductID,
	// 		})
	// 	}
	// }

	return products, nil
}

// Exchange operation
func (api *AlpacaAPI) FetchOrdersFills() ([]Order, error) {
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
func (api *AlpacaAPI) FetchCandles(productID string, timeframe Timeframe, start, end time.Time) ([]Candle, error) {
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
		return fetch_Alpaca_Candles(productID, timeframe, start, end)
	}

	// Otherwise, split the request into multiple calls
	currentStart := start
	for currentStart.Before(end) {
		currentEnd := currentStart.Add(time.Duration(maxCandles) * candleDuration)
		if currentEnd.After(end) {
			currentEnd = end
		}

		res, err := fetch_Alpaca_Candles(productID, timeframe, currentStart, currentEnd)
		if err != nil {
			return nil, err
		}

		candles = append(candles, res...)
		currentStart = currentEnd
	}
	return candles, nil
}

func fetch_Alpaca_Candles(productID string, timeframe Timeframe, start, end time.Time) ([]Candle, error) {
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
func (api *AlpacaAPI) FetchFills() ([]Fill, error) {
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
func (api *AlpacaAPI) FetchPortfolio() ([]Asset, error) {
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
			price, _ := GetPrice(acct.Asset)
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

// func GetCBSign(apiSecret string, timestamp int64, method, path, body string) string {
// 	message := fmt.Sprintf("%d%s%s%s", timestamp, method, path, body)
// 	hasher := hmac.New(sha256.New, []byte(apiSecret))
// 	hasher.Write([]byte(message))
// 	signature := hex.EncodeToString(hasher.Sum(nil))
// 	return signature
// }

// func GetPrice(currency string) (float64, error) {
// 	if currency == "USD" || currency == "USDT" || currency == "USDC" {
// 		return 1.0, nil
// 	}

// 	apiKey := os.Getenv("CBAPIKEY")
// 	apiSecret := os.Getenv("CBAPISECRET")

// 	timestamp := time.Now().Unix()
// 	baseURL := "https://api.coinbase.com"
// 	path := fmt.Sprintf("/api/v3/brokerage/products/%s-USD", currency)
// 	method := "GET"
// 	body := ""

// 	signature := GetCBSign(apiSecret, timestamp, method, path, body)

// 	client := &http.Client{}
// 	req, err := http.NewRequest(method, baseURL+path, nil)
// 	if err != nil {
// 		return 0, fmt.Errorf("NewRequest: %v", err)
// 	}

// 	req.Header.Add("CB-ACCESS-SIGN", signature)
// 	req.Header.Add("CB-ACCESS-TIMESTAMP", strconv.FormatInt(timestamp, 10))
// 	req.Header.Add("CB-ACCESS-KEY", apiKey)
// 	req.Header.Add("CB-VERSION", "2015-07-22")

// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return 0, fmt.Errorf("Error DO: %v", err)
// 	}
// 	defer resp.Body.Close()

// 	var tickerResponse struct {
// 		Price string `json:"price"`
// 	}

// 	err = json.NewDecoder(resp.Body).Decode(&tickerResponse)
// 	if err != nil {
// 		return 0, fmt.Errorf("Error decoding JSON: %v", err)
// 	}

// 	price, err := strconv.ParseFloat(tickerResponse.Price, 64)
// 	if err != nil {
// 		return 0, fmt.Errorf("Error parsing price: %v", err)
// 	}

// 	return price, nil
// }
