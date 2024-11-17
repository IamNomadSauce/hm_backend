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

type CoinbaseOrderResponse struct {
	OrderID            string             `json:"order_id"`
	ProductID          string             `json:"product_id"`
	OrderConfiguration OrderConfiguration `json:"order_configuration"`
	Side               string             `json:"side"`
	Status             string             `json:"status"`
	CreatedTime        string             `json:"created_time"`
	FilledSize         string             `json:"filled_size"`
	AverageFilledPrice string             `json:"average_filled_price"`
	TotalFees          string             `json:"total_fees"`
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
	// fmt.Printf("Raw response: %s\n", string(responseBody))

	var response struct {
		Orders []struct {
			OrderID            string `json:"order_id"`
			ProductID          string `json:"product_id"`
			Side               string `json:"side"`
			Status             string `json:"status"`
			CreatedTime        string `json:"created_time"`
			FilledSize         string `json:"filled_size"`
			AverageFilledPrice string `json:"average_filled_price"`
			TotalFees          string `json:"total_fees"`
			OrderConfiguration struct {
				MarketMarketIoc *struct {
					QuoteSize string `json:"quote_size"`
					BaseSize  string `json:"base_size"`
				} `json:"market_market_ioc"`
				LimitLimitGtc *struct {
					BaseSize   string `json:"base_size"`
					LimitPrice string `json:"limit_price"`
					PostOnly   bool   `json:"post_only"`
				} `json:"limit_limit_gtc"`
				// Add other order types as needed
			} `json:"order_configuration"`
		} `json:"orders"`
	}

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling JSON: %w", err)
	}

	var filteredOrders []Order
	for _, cbOrder := range response.Orders {
		if cbOrder.Status != "CANCELLED" && cbOrder.Status != "FILLED" {
			// Parse size and price based on order configuration
			var size, price float64

			if cbOrder.OrderConfiguration.MarketMarketIoc != nil {
				size, _ = strconv.ParseFloat(cbOrder.OrderConfiguration.MarketMarketIoc.BaseSize, 64)
				// For market orders, use average filled price
				price, _ = strconv.ParseFloat(cbOrder.AverageFilledPrice, 64)
			} else if cbOrder.OrderConfiguration.LimitLimitGtc != nil {
				size, _ = strconv.ParseFloat(cbOrder.OrderConfiguration.LimitLimitGtc.BaseSize, 64)
				price, _ = strconv.ParseFloat(cbOrder.OrderConfiguration.LimitLimitGtc.LimitPrice, 64)
			}

			filledSize, _ := strconv.ParseFloat(cbOrder.FilledSize, 64)
			totalFees, _ := strconv.ParseFloat(cbOrder.TotalFees, 64)

			order := Order{
				OrderID:        cbOrder.OrderID,
				ProductID:      cbOrder.ProductID,
				Side:           cbOrder.Side,
				Status:         cbOrder.Status,
				Price:          price,
				Size:           size,
				FilledSize:     filledSize,
				TotalFees:      totalFees,
				Timestamp:      cbOrder.CreatedTime,
				MarketCategory: "crypto_spot",
				XchID:          api.ExchangeID,
			}

			// log.Printf("Created order - Price: %v, Size: %v, Time: %v", order.Price, order.Size, order.Timestamp)
			filteredOrders = append(filteredOrders, order)
		}
	}

	return filteredOrders, nil
}

func ParseCoinbaseOrder(response CoinbaseOrderResponse) (Order, error) {
	price, _ := strconv.ParseFloat(response.AverageFilledPrice, 64)
	filledSize, _ := strconv.ParseFloat(response.FilledSize, 64)
	totalFees, _ := strconv.ParseFloat(response.TotalFees, 64)

	// Get size and price based on order configuration type
	var size float64
	if response.OrderConfiguration.MarketMarketIoc != nil {
		size, _ = strconv.ParseFloat(response.OrderConfiguration.MarketMarketIoc.BaseSize, 64)
	} else if response.OrderConfiguration.LimitLimitGtc != nil {
		size, _ = strconv.ParseFloat(response.OrderConfiguration.LimitLimitGtc.BaseSize, 64)
	}

	return Order{
		OrderID:    response.OrderID,
		ProductID:  response.ProductID,
		Timestamp:  response.CreatedTime,
		Side:       response.Side,
		Status:     response.Status,
		Price:      price,
		Size:       size,
		FilledSize: filledSize,
		TotalFees:  totalFees,
		// ... set other fields
	}, nil
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

	type CBFill struct {
		EntryID             string `json:"entry_id"`
		TradeID             string `json:"trade_id"`
		OrderID             string `json:"order_id"`
		TradeTime           string `json:"trade_time"`
		TradeType           string `json:"trade_type"`
		Price               string `json:"price"`
		Size                string `json:"size"`
		Commission          string `json:"commission"`
		ProductID           string `json:"product_id"`
		Sequence_Timestamp  string `json:"sequence_timestamp"`
		Liquidity_Indicator string `json:"liquidity_indicator"`
		SizeInQuote         bool   `json:"size_in_quote"`
		UserID              string `json:"user_id"`
		Side                string `json:"side"`
		RetailPortfolioID   string `json:"retail_portfolio_id"`
	}

	var response struct {
		Fills []CBFill `json:"fills"`
	}

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling JSON: %w", err)
	}

	for _, fill := range response.Fills {
		price, _ := strconv.ParseFloat(fill.Price, 64)
		size, _ := strconv.ParseFloat(fill.Size, 64)
		commission, _ := strconv.ParseFloat(fill.Commission, 64)
		converted_fill := Fill{
			EntryID:        fill.EntryID,
			TradeID:        fill.TradeID,
			OrderID:        fill.OrderID,
			Timestamp:      fill.TradeTime,
			Price:          price,
			Size:           size,
			Side:           fill.Side,
			Commission:     commission,
			ProductID:      fill.ProductID,
			XchID:          api.ExchangeID,
			MarketCategory: "Spot Crypto",
		}
		fills = append(fills, converted_fill)
	}

	return fills, nil
}

type CoinbaseAccount struct {
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
	PortfolioID      string  `json:"retail_portfolio_id"`
	Size             float64 `json:"size"` // Changed from string to float64
}

// Exchange operation
func (api *CoinbaseAPI) FetchPortfolio() ([]Asset, error) {
	log.Println("FetchPortfolio")
	// var accounts []Asset
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
		Accounts []CoinbaseAccount `json:"accounts"`
		HasNext  bool              `json:"has_next"`
		Cursor   string            `json:"cursor"`
		Size     float64           `json:"size"` // Changed from string to float64
	}

	// Print raw response for debugging
	fmt.Printf("Raw response: %s\n", string(responseBody))

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshaling JSON: %w", err)
	}

	var assets []Asset
	for _, account := range response.Accounts {
		availableBalance, _ := strconv.ParseFloat(account.AvailableBalance.Value, 64)
		holdBalance, _ := strconv.ParseFloat(account.Hold.Value, 64)

		if availableBalance > 0 || holdBalance > 0 {
			var totalValue float64

			// Handle USD and stablecoins
			if account.Currency == "USD" || account.Currency == "USDT" || account.Currency == "USDC" {
				totalValue = availableBalance + holdBalance
			} else {
				// Get price for other cryptocurrencies
				price, err := GetPrice(account.Currency + "-USD")
				if err != nil {
					log.Printf("Error getting price for %s: %v", account.Currency, err)
					continue
				}
				totalValue = (availableBalance + holdBalance) * price
			}

			asset := Asset{
				Asset:            account.Currency + "-USD",
				AvailableBalance: account.AvailableBalance,
				Hold:             account.Hold,
				Value:            totalValue,
				XchID:            api.ExchangeID,
			}

			assets = append(assets, asset)
			log.Printf("Added asset %s: Available=%v Hold=%v Value=%v",
				account.Currency, availableBalance, holdBalance, totalValue)
		}
	}

	return assets, nil
}
func GetPrice(currency string) (float64, error) {
	if currency == "USD" || currency == "USDT" || currency == "USDC" {
		return 1.0, nil
	}

	apiKey := os.Getenv("CBAPIKEY")
	apiSecret := os.Getenv("CBAPISECRET")

	timestamp := time.Now().Unix()
	baseURL := "https://api.coinbase.com"
	path := fmt.Sprintf("/api/v3/brokerage/products/%s", currency)
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

	// Read response body as []byte
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("Error reading response body: %v", err)
	}

	// Log response for debugging
	// log.Printf("Price response for %s: %s", currency, string(bodyBytes))

	var productResponse struct {
		Price          string `json:"price"`
		ProductID      string `json:"product_id"`
		BaseIncrement  string `json:"base_increment"`
		QuoteIncrement string `json:"quote_increment"`
	}

	// Use bodyBytes directly for unmarshaling
	err = json.Unmarshal(bodyBytes, &productResponse)
	if err != nil {
		return 0, fmt.Errorf("Error decoding JSON: %v", err)
	}

	if productResponse.Price == "" {
		return 0, fmt.Errorf("No price available for %s", currency)
	}

	price, err := strconv.ParseFloat(productResponse.Price, 64)
	if err != nil {
		return 0, fmt.Errorf("Error parsing price: %v", err)
	}

	return price, nil
}

func GetCBSign(apiSecret string, timestamp int64, method, path, body string) string {
	message := fmt.Sprintf("%d%s%s%s", timestamp, method, path, body)
	hasher := hmac.New(sha256.New, []byte(apiSecret))
	hasher.Write([]byte(message))
	signature := hex.EncodeToString(hasher.Sum(nil))
	return signature
}
