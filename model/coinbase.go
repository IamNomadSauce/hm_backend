package model

import (
	"backend/alerts"
	"bytes"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"

	// _ "hm/alerts"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/websocket"
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
	WSConn              *websocket.Conn
	UserWSConn          *websocket.Conn
	alertManager        *alerts.AlertManager
}

func (api *CoinbaseAPI) ConnectUserWebsocket() error {
	url := "wss://advanced-trade-ws-user.coinbase.com"
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("coinbase User WebSocket dial error: %v", err)
	}
	api.UserWSConn = c

	// Generate JWT token
	jwtToken, err := generateUserJWT(os.Getenv("CBAPIKEYNAME"), []byte(os.Getenv("CBAPIUSERSECRET")))
	if err != nil {
		return fmt.Errorf("JWT generation error: %v", err)
	}

	subscribeMsg := struct {
		Type    string `json:"type"`
		Channel string `json:"channel"`
		JWT     string `json:"jwt"`
	}{
		Type:    "subscribe",
		Channel: "user",
		JWT:     jwtToken,
	}

	if err := c.WriteJSON(subscribeMsg); err != nil {
		return fmt.Errorf("user WebSocket subscription error: %v", err)
	}

	// Subscribe to heartbeats channel
	heartbeatMsg := struct {
		Type    string `json:"type"`
		Channel string `json:"channel"`
		JWT     string `json:"jwt"`
	}{
		Type:    "subscribe",
		Channel: "heartbeats",
		JWT:     jwtToken,
	}

	if err := c.WriteJSON(heartbeatMsg); err != nil {
		return fmt.Errorf("heartbeat subscription error: %v", err)
	}

	// log.Printf("Debug - Sending user subscription message: %+v", subscribeMsg)

	go api.handleUserWebsocketMessages()
	go api.monitorUserHeartbeat()

	return nil
}

func (api *CoinbaseAPI) generateJWT() string {
	timestamp := time.Now().Unix()
	log.Println("Timestamp", timestamp, reflect.TypeOf(timestamp))
	message := fmt.Sprintf("%d%s%s", timestamp, "GET", "/ws/auth")
	h := hmac.New(sha256.New, []byte(api.APISecret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

func generateUserJWT(apiKeyName string, privateKeyBytes []byte) (string, error) {
	block, _ := pem.Decode(privateKeyBytes)
	if block == nil {
		return "", fmt.Errorf("invalid PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		key, err = x509.ParseECPrivateKey(block.Bytes) // Try EC key parsing if PKCS8 fails
		if err != nil {
			return "", fmt.Errorf("failed to parse private key: %v", err)
		}
	}

	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("key is not an ECDSA private key")
	}

	now := time.Now().Unix()
	claims := jwt.MapClaims{
		"iss": "coinbase-cloud",
		"sub": apiKeyName,
		"nbf": now,
		"exp": now + 120,
		"uri": "wss://advanced-trade-ws-user.coinbase.com",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = apiKeyName
	token.Header["nonce"] = generateNonce()

	return token.SignedString(ecKey)
}

func generateNonce() string {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(nonce)
}

func (api *CoinbaseAPI) handleUserWebsocketMessages() {
	orderStatuses := make(map[string]string)
	for {
		_, message, err := api.UserWSConn.ReadMessage()
		if err != nil {
			log.Printf("User WebSocket read error: %v", err)
			return
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("JSON unmarshal error: %v", err)
			continue
		}

		if events, ok := msg["events"].([]interface{}); ok {
			for _, event := range events {
				if eventMap, ok := event.(map[string]interface{}); ok {
					eventType, exists := eventMap["type"]
					if !exists {
						continue
					}

					switch eventType {
					case "snapshot":
						// ... existing
						// Handle orders snapshot
						// if orders, ok := eventMap["orders"].([]interface{}); ok {
						// 	log.Printf("\n=== Initial Orders Snapshot ===")
						// 	for _, order := range orders {
						// 		if o, ok := order.(map[string]interface{}); ok {
						// 			orderID := o["order_id"].(string)
						// 			status := o["status"].(string)
						// 			orderStatuses[orderID] = status
						// 			// log.Printf("Order: ID=%v, Side=%v, Product=%v, Status=%v, Price=%v, Size=%v", orderID, o["order_side"], o["product_id"],status, o["limit_price"], o["leaves_quantity"])
						// 		}
						// 	}
						// }

						// Handle positions snapshot
						// if positions, ok := eventMap["positions"].(map[string]interface{}); ok {
						// log.Printf("\n=== Initial Positions Snapshot ===")
						// log.Println(eventMap["positions"])

						// Handle perpetual futures positions
						// if perpetual, ok := positions["perpetual_futures_positions"].([]interface{}); ok && len(perpetual) > 0 {
						// 	log.Printf("\nPerpetual Futures Positions:")
						// 	for _, pos := range perpetual {
						// 		if p, ok := pos.(map[string]interface{}); ok {
						// 			log.Printf("Product: %v, Side: %v, Size: %v, Entry Price: %v, Mark Price: %v, PnL: %v",
						// 				p["product_id"],
						// 				p["position_side"],
						// 				p["net_size"],
						// 				p["entry_vwap"],
						// 				p["mark_price"],
						// 				p["unrealized_pnl"])
						// 		}
						// 	}
						// }

						// Handle expiring futures positions
						// if expiring, ok := positions["expiring_futures_positions"].([]interface{}); ok && len(expiring) > 0 {
						// 	log.Printf("\nExpiring Futures Positions:")
						// 	for _, pos := range expiring {
						// 		if p, ok := pos.(map[string]interface{}); ok {
						// 			log.Printf("Product: %v, Side: %v, Contracts: %v, Entry Price: %v, PnL: %v",
						// 				p["product_id"],
						// 				p["side"],
						// 				p["number_of_contracts"],
						// 				p["entry_price"],
						// 				p["unrealized_pnl"])
						// 		}
						// 	}
						// }

						// If no positions found
						// 	if len(positions) == 0 {
						// 		log.Printf("No open positions found")
						// 	}
						// }

					case "update":
						if orders, ok := eventMap["orders"].([]interface{}); ok {
							for _, order := range orders {
								if o, ok := order.(map[string]interface{}); ok {
									orderID := o["order_id"].(string)
									newStatus := o["status"].(string)
									if lastStatus, exists := orderStatuses[orderID]; !exists || lastStatus != newStatus {
										orderStatuses[orderID] = newStatus
										log.Printf("\n=== Order Update ===")
										log.Printf("ID: %v, Side: %v, Status: %v, Product: %v",
											orderID, o["order_side"], newStatus, o["product_id"])
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

func (api *CoinbaseAPI) ConnectMarketDataWebSocket() error {
	url := "wss://advanced-trade-ws.coinbase.com"
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("market Data WebSocket dial error: %v", err)
	}
	api.WSConn = c

	// First subscribe to ticker channel (no auth needed)
	tickerMsg := struct {
		Type       string   `json:"type"`
		ProductIDs []string `json:"product_ids"`
		Channel    string   `json:"channel"`
	}{
		Type:       "subscribe",
		ProductIDs: []string{"BTC-USD", "XLM-USD"},
		Channel:    "ticker",
	}

	if err := c.WriteJSON(tickerMsg); err != nil {
		return fmt.Errorf("WebSocket ticker subscription error: %v", err)
	}
	// log.Printf("Debug - Sending ticker subscription message: %+v", tickerMsg)

	// Then subscribe to heartbeats channel (requires auth)
	heartbeatMsg := struct {
		Type       string   `json:"type"`
		ProductIDs []string `json:"product_ids"`
		Channel    string   `json:"channel"`
		JWT        string   `json:"jwt"`
	}{
		Type:       "subscribe",
		ProductIDs: []string{"BTC-USD", "XLM-USD"},
		Channel:    "heartbeats",
		JWT:        api.generateJWT(), // You'll need to implement this
	}

	// log.Printf("Debug - Sending heartbeat subscription message: %+v", heartbeatMsg)
	if err := c.WriteJSON(heartbeatMsg); err != nil {
		return fmt.Errorf("WebSocket heartbeat subscription error: %v", err)
	}

	go api.handleWebsocketMessages()
	// go api.monitorHeartbeat()

	return nil
}

func (api *CoinbaseAPI) handleWebsocketMessages() {
	for {
		_, message, err := api.WSConn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			return
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("JSON unmarshal error: %v", err)
			continue
		}

		// log.Printf("Debug - Received message: %+v", msg)

		switch msg["channel"] {
		case "error":
			log.Printf("WebSocket error received: %v", msg["message"])
			return
		case "subscribe":
			log.Printf("Subscription confirmed for channels: %v", msg["channels"])
		case "heartbeat":
			log.Printf("Heartbeat received for: %v", msg["product_ids"])
		case "ticker":
			productID := msg["product_id"].(string)
			price, _ := strconv.ParseFloat(msg["product_id"].(string), 64)
			triggeredAlerts := api.alertManager.ProcessPriceAlerts(productID, price)

			for _, alert := range triggeredAlerts {
				log.Println("Alert Triggered", alert)
				// if err := api.alertManager.UpdateAlertStatus(alert.ID, "triggered"); err != nil {
				// 	log.Printf("Error updating alert status: %v", err)
				// }
			}

		}
	}
}

func (api *CoinbaseAPI) monitorUserHeartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := api.UserWSConn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Printf("Failed to send user websocket ping: %v", err)
				api.reconnectUserWebSocket()
				return
			}
		}
	}
}

func (api *CoinbaseAPI) reconnectUserWebSocket() {
	log.Println("Attempting to reconnect user websocket...")
	api.UserWSConn.Close()

	backoff := time.Second
	maxBackoff := time.Minute * 2

	for {
		if err := api.ConnectUserWebsocket(); err != nil {
			log.Printf("Failed to reconnect user websocket: %v", err)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		log.Println("Successfully reconnected user websocket")
		return
	}
}

// func (api *CoinbaseAPI) monitorHeartbeat() {
// 	ticker := time.NewTicker(30 * time.Second)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ticker.C:
// 			if err := api.WSConn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
// 				log.Printf("Failed to send ping: %v", err)
// 				api.reconnectWebSocket()
// 				return
// 			}
// 		}
// 	}
// }

// func (api *CoinbaseAPI) reconnectWebSocket() {
// 	time.Sleep(5 * time.Second)
// 	if err := api.ConnectWebSocket(); err != nil {
// 		log.Printf("Failed to reconnect WebSocket: %v", err)
// 	}
// }

func (api *CoinbaseAPI) handleTickerUpdate(msg map[string]interface{}) {
	log.Println("Handle Ticker Update", msg)
}

func (api *CoinbaseAPI) handleOrderUpdate(msg map[string]interface{}) {
	log.Println("Handle Order Update", msg)
	// Process order and update DB
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
			// log.Println(cbOrder)

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

func (api *CoinbaseAPI) PlaceBracketOrder(trade_group Trade) error {
	log.Println("Coinbase API Place Bracket Order", trade_group)
	entryOrderBody := struct {
		ClientOrderID      string `json:"client_order_id"`
		ProductID          string `json:"product_id"`
		Side               string `json:"side"`
		OrderConfiguration struct {
			LimitLimitGtc struct {
				BaseSize   string `json:"base_size"`
				LimitPrice string `json:"limit_price"`
				PostOnly   bool   `json:"post_only"`
			} `json:"limit_limit_gtc"`
		} `json:"order_configuration"`
	}{
		ClientOrderID: fmt.Sprintf("%d", time.Now().UnixNano()),
		ProductID:     trade_group.ProductID,
		Side:          trade_group.Side,
	}

	entryOrderBody.OrderConfiguration.LimitLimitGtc.BaseSize = fmt.Sprintf("%.8f", trade_group.Size)
	entryOrderBody.OrderConfiguration.LimitLimitGtc.LimitPrice = fmt.Sprintf("%.8f", trade_group.Size)

	entryOrderID, err := api.PlaceOrder(trade_group)
	if err != nil {
		return fmt.Errorf("failed to place entry order: %w", err)
	}

	fmt.Println("Entry Order ID", entryOrderID)

	go func() {
		for {
			order, err := api.GetOrder(entryOrderID)
			if err != nil {
				log.Printf("Error checking entry order status: %v", err)
				continue
			}

			if order.Status == "FILLED" {
				fmt.Println("Order filled, creating bracket order")
				exitSide := "SELL"
				if trade_group.Side == "SELL" {
					exitSide = "BUY"
				}

				bracketBody := struct {
					ClientOrderID      string `json:"client_order_id"`
					ProductID          string `json:"product_id"`
					Side               string `json:"side"`
					OrderConfiguration struct {
						TriggerBracketGTD struct {
							BaseSize         string `json:"base_size"`
							LimitPrice       string `json:"limit_price"`
							StopTriggerPrice string `json:"stop_trigger_price"`
							EndTime          string `json:"end_time"`
						} `json:"trigger_bracket_gtd"`
					} `json:"order_configuration"`
				}{
					ClientOrderID: fmt.Sprintf("%d", time.Now().UnixNano()),
					ProductID:     trade_group.ProductID,
					Side:          exitSide,
				}

				bracketBody.OrderConfiguration.TriggerBracketGTD.BaseSize = fmt.Sprintf("%.8f", trade_group.Size)
				bracketBody.OrderConfiguration.TriggerBracketGTD.LimitPrice = fmt.Sprintf("%.8f", trade_group.PTPrice)
				bracketBody.OrderConfiguration.TriggerBracketGTD.StopTriggerPrice = fmt.Sprintf("%.8f", trade_group.StopPrice)
				bracketBody.OrderConfiguration.TriggerBracketGTD.EndTime = time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)

				_, err = api.PlaceOrder(trade_group)
				if err != nil {
					log.Printf("Error placing bracket orders: %v", err)
				}
				return
			} else {
				fmt.Println("Waiting on order to be filled")
			}
			time.Sleep(5 * time.Second)
		}
	}()

	return nil
}

func (api *CoinbaseAPI) PlaceOrder(trade Trade) (string, error) {
	orderBody := struct {
		ClientOrderID      string `json:"client_order_id"`
		ProductID          string `json:"product_id"`
		Side               string `json:"side"`
		OrderConfiguration struct {
			LimitLimitGtc struct {
				BaseSize   string `json:"base_size"`
				LimitPrice string `json:"limit_price"`
				PostOnly   bool   `json:"post_only"`
			} `json:"limit_limit_gtc"`
		} `json:"order_configuration"`
	}{
		ClientOrderID: fmt.Sprintf("%d", time.Now().UnixNano()),
		ProductID:     trade.ProductID,
		Side:          trade.Side,
	}

	// Format size and price with proper precision (6 decimal places for XLM)
	orderBody.OrderConfiguration.LimitLimitGtc.BaseSize = fmt.Sprintf("%.2f", trade.Size)
	orderBody.OrderConfiguration.LimitLimitGtc.LimitPrice = fmt.Sprintf("%.6f", trade.EntryPrice)
	orderBody.OrderConfiguration.LimitLimitGtc.PostOnly = false

	bodyBytes, err := json.Marshal(orderBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request body: %w", err)
	}

	log.Printf("Request body: %s", string(bodyBytes))

	timestamp := time.Now().Unix()
	path := "/api/v3/brokerage/orders"
	method := "POST"

	signature := GetCBSign(api.APISecret, timestamp, method, path, string(bodyBytes))

	req, err := http.NewRequest(method, api.BaseURL+path, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("CB-ACCESS-SIGN", signature)
	req.Header.Add("CB-ACCESS-TIMESTAMP", strconv.FormatInt(timestamp, 10))
	req.Header.Add("CB-ACCESS-KEY", api.APIKey)
	req.Header.Add("CB-VERSION", "2015-07-22")
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	log.Printf("Response body: %s", string(respBody))

	var response struct {
		Success       bool `json:"success"`
		ErrorResponse struct {
			Error        string `json:"error"`
			Message      string `json:"message"`
			ErrorDetails string `json:"error_details"`
		} `json:"error_response"`
		OrderID string `json:"order_id"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}

	if !response.Success {
		return "", fmt.Errorf("order placement failed: %s - %s",
			response.ErrorResponse.Error, response.ErrorResponse.Message)
	}

	return response.OrderID, nil
}

func (api *CoinbaseAPI) GetOrder(orderID string) (*Order, error) {
	timestamp := time.Now().Unix()
	path := fmt.Sprintf("/api/v3/brokerage/orders/get_order?order_id=%s", orderID)
	method := "GET"

	signature := GetCBSign(api.APISecret, timestamp, method, path, "")

	req, err := http.NewRequest(method, api.BaseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating get order request: %w", err)
	}

	req.Header.Add("CB-ACCESS-SIGN", signature)
	req.Header.Add("CB-ACCESS-TIMESTAMP", strconv.FormatInt(timestamp, 10))
	req.Header.Add("CB-ACCESS-KEY", api.APIKey)
	req.Header.Add("CB-VERSION", "2015-07-22")
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error executing get order request: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("Getting order status for ID: %s", orderID)

	var response struct {
		Order struct {
			OrderID            string `json:"order_id"`
			ProductID          string `json:"product_id"`
			Side               string `json:"side"`
			Status             string `json:"status"`
			CreatedTime        string `json:"created_time"`
			CompletedTime      string `json:"completed_time"`
			FilledSize         string `json:"filled_size"`
			OrderType          string `json:"order_type"`
			OrderConfiguration struct {
				LimitLimitGtc struct {
					BaseSize   string `json:"base_size"`
					LimitPrice string `json:"limit_price"`
				} `json:"limit_limit_gtc"`
			} `json:"order_configuration"`
		} `json:"order"`
		Success bool `json:"success"`
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response body: %w", err)
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("Error decoding order response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("Unsuccessful order query")
	}

	log.Printf("Order %s status: %s", orderID, response.Order.Status)

	size, _ := strconv.ParseFloat(response.Order.OrderConfiguration.LimitLimitGtc.BaseSize, 64)
	price, _ := strconv.ParseFloat(response.Order.OrderConfiguration.LimitLimitGtc.LimitPrice, 64)
	filledSize, _ := strconv.ParseFloat(response.Order.FilledSize, 64)

	return &Order{
		OrderID:    response.Order.OrderID,
		ProductID:  response.Order.ProductID,
		Side:       response.Order.Side,
		Status:     response.Order.Status,
		Size:       size,
		Price:      price,
		FilledSize: filledSize,
		// OrderType: response.Order.OrderType,
		Timestamp: response.Order.CreatedTime,
	}, nil
}

// Exchange operation
func (api *CoinbaseAPI) FetchPortfolio() ([]Asset, error) {
	// log.Println("FetchPortfolio")
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
	// fmt.Printf("Raw response: %s\n", string(responseBody))

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
			// log.Printf("Added asset %s: Available=%v Hold=%v Value=%v", account.Currency, availableBalance, holdBalance, totalValue)
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
