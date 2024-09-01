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













