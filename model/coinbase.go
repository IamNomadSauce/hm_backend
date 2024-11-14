package model

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type CoinbaseAPI struct {
	APIKey              string
	APISecret           string
	BaseURL             string
	RateLimit           int
	CandleLimit         int
	RateWindow          time.Duration
	RequestCount        int
	LastRequest         time.Time
	ExchangeID          int
	SupportedOrderTypes []string
	SupportedTimeframes []string
	MinimumOrderSizes   map[string]float64
	MakerFee            float64
	TakerFee            float64
}

type CoinbaseOrder struct {
	OrderID       string `json:"order_id"`
	ClientOrderID string `json:"client_order_id"`
	ProductID     string `json:"product_id"`
	Side          string `json:"side"`
	Status        string `json:"status"`
	TimeInForce   string `json:"time_in_force"`
	CreatedTime   string `json:"created_time"`
	CompletedTime string `json:"completed_time"`
	OrderType     string `json:"order_type"`
	Size          string `json:"size"` // Changed to string
	FilledSize    string `json:"filled_size"`
	Price         string `json:"price"`      // Changed to string
	TotalFees     string `json:"total_fees"` // Changed to string
}

// Exchange operation
func (api *CoinbaseAPI) FetchOrdersFills() ([]Order, error) {
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

	// Debug: Print raw response
	fmt.Printf("Raw response: %s\n", string(responseBody))

	var response struct {
		Orders []CoinbaseOrder `json:"orders"`
	}

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling JSON: %w", err)
	}

	var filteredOrders []Order
	for _, cbOrder := range response.Orders {
		if cbOrder.Status != "CANCELLED" {
			// Convert CoinbaseOrder to your Order struct
			price, _ := strconv.ParseFloat(cbOrder.Price, 64)
			log.Println(cbOrder)
			order := Order{
				OrderID:        cbOrder.OrderID,
				ProductID:      cbOrder.ProductID,
				Side:           cbOrder.Side,
				Status:         cbOrder.Status,
				Price:          price,
				Size:           toNullFloat64(cbOrder.Size), // Use toNullFloat64
				FilledSize:     cbOrder.FilledSize,
				TotalFees:      toNullFloat64(cbOrder.TotalFees), // Use toNullFloat64
				Timestamp:      parseTimestamp(cbOrder.CreatedTime),
				MarketCategory: "crypto_spot",
				XchID:          api.ExchangeID,
			}

			filteredOrders = append(filteredOrders, order)
		}
	}

	log.Printf("API:Orders: %d", len(filteredOrders))
	return filteredOrders, nil
}
func toNullFloat64(s string) sql.NullFloat64 {
	if s == "" {
		return sql.NullFloat64{Valid: false}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: f, Valid: true}
}

// Helper function to parse Coinbase timestamp
func parseTimestamp(timeStr string) int64 {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		log.Printf("Error parsing timestamp %s: %v", timeStr, err)
		return 0
	}
	return t.Unix()
}

func (api *CoinbaseAPI) FetchAvailableProducts() ([]Product, error) {
	fmt.Println("Fetch Available Products")
	if api == nil {
		return nil, fmt.Errorf("CoinbaseAPI is not initialized")
	}

	path := "/api/v3/brokerage/products"
	method := "GET"

	// Construct the full URL
	fullURL := fmt.Sprintf("%s%s", api.BaseURL, path)

	// Create timestamp for authentication
	timestamp := time.Now().Unix()
	// secret := os.Getenv("CBAPISECRET")
	signature := GetCBSign(api.APISecret, timestamp, method, path, "")

	// fmt.Printf("SECRET: |%s|", api.APISecret)
	// fmt.Printf("SECRET: |%s|", secret)

	// Create new request
	req, err := http.NewRequest(method, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Add headers
	req.Header.Add("CB-ACCESS-SIGN", signature)
	req.Header.Add("CB-ACCESS-TIMESTAMP", strconv.FormatInt(timestamp, 10))
	req.Header.Add("CB-ACCESS-KEY", api.APIKey)
	req.Header.Add("CB-VERSION", "2015-07-22")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		// fmt.Println("Body:", resp)
		return nil, fmt.Errorf("error response from Coinbase: %d - %s", resp.StatusCode, string(body))
	}

	// Read and parse the response
	var response struct {
		Products []Product
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	// Convert to our Product type
	var products []Product
	for _, p := range response.Products {
		if p.Status == "online" { // Only include active products
			products = append(products, Product{
				ID:         p.ID,    // Auto generated db id
				XchID:      p.XchID, // Specific exchange id.
				ProductID:  p.ProductID,
				BaseName:   p.BaseName,
				QuoteName:  p.QuoteName,
				Status:     p.Status,
				Price:      p.Price,
				Volume_24h: p.Volume_24h,
			})
		}
	}
	// Sort products by 24h volume in descending order
	sort.Slice(products, func(i, j int) bool {
		// Convert volume strings to float64 for comparison
		volI, _ := strconv.ParseFloat(products[i].Volume_24h, 64)
		volJ, _ := strconv.ParseFloat(products[j].Volume_24h, 64)
		return volI > volJ
	})

	// Take only the top 100 products (or less if fewer products exist)
	maxProducts := 100
	if len(products) > maxProducts {
		products = products[:maxProducts]
	}
	fmt.Println("Products: ", len(products))

	return products, nil
}

// Exchange operation
func (api *CoinbaseAPI) FetchCandles(productID string, timeframe Timeframe, start, end time.Time) ([]Candle, error) {
	var candles []Candle
	fmt.Println("\n-------------------------\nCoinbaseAPI.FetchCandles\n", productID, "\n", timeframe.Endpoint, "\n", start, "\n", end, "\n")
	fmt.Println("APIKEY", api.BaseURL)

	fmt.Println(api.CandleLimit)

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
	fmt.Println("\n-------------------------\nfetch_Coinbase_Candles \n", productID, "\n", timeframe.Endpoint, "\n", start, "\n", end, "\n")
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
			price, _ := GetPrice(acct.Symbol.ProductID)
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
