package model

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"time"
)

type Candle struct {
	Timestamp int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

type Timeframe struct {
	ID       int64  `db:"id"`
	XchID    int64  `db:"xch_id"`
	TF       string `db:"label"`
	Endpoint string `db:"endpoint"`
	Minutes  int64  `db:"minutes"`
}

type Exchange struct {
	ID                int
	Name              string
	Timeframes        []Timeframe
	Orders            []Order
	Fills             []Fill
	Watchlist         []Product
	CandleLimit       int64
	API               ExchangeAPI
	AvailableProducts []Product
}

type ExchangeAPI interface {
	FetchOrdersFills() ([]Order, error)
	FetchFills() ([]Fill, error)
	FetchPortfolio() ([]Asset, error)
	FetchCandles(product string, timeframe Timeframe, start, end time.Time) ([]Candle, error)
	FetchAvailableProducts() ([]Product, error)
}

type Product struct {
	ID                int    `json:"id"`
	XchID             int    `json:"xch_id"`
	ProductID         string `json:"product_id"`
	BaseName          string `json:"base_name"`
	QuoteName         string `json:"quote_name"`
	Status            string `json:"status"`
	Price             string `json:"price"`
	Volume_24h        string `json:"volume_24"`
	Base_Currency_ID  string `json:"base_currency_id"`
	Quote_Currency_ID string `json:"quote_currency_id"`
}

type Asset struct {
	Symbol           Product
	AvailableBalance Balance
	Hold             Balance
	Value            float64
}

type Balance struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

type Watchlist struct {
	ID      int    `db:"id"`
	Product string `db:"product"`
	XchID   int    `db:"xch_id"`
}

type Order struct {
	Timestamp      int64  `db:"time"`
	OrderID        string `db:"order_id"`   // Exchange specific order identifier
	ProductID      string `db:"product_id"` // xbt_usd_15
	TradeType      string `db:"trade_type"` // Long / Short
	Side           string `db:"side"`       // buy / sell
	XchID          int    `db:"xch_id"`
	MarketCategory string `db:"market_category"` // (crypto / equities)_(spot / futures)
	Price          string `db:"price"`           // instrument_currency
	Size           string `db:"size"`            // How many of instrument
	FilledSize     string `db:"filled_size"`
	Status         string `db:"status"`
	TotalFees      string `json:"total_fees"`
}

type Fill struct {
	EntryID        string         `db:"entry_id"` // Changed from Timestamp
	TradeID        string         `db:"trade_id"`
	OrderID        string         `db:"order_id"`
	Timestamp      int64          `db:"time"` // Moved Timestamp
	TradeType      string         `db:"trade_type"`
	Price          sql.NullString `db:"price"`
	Size           sql.NullString `db:"size"`
	Side           string         `db:"side"`
	Commission     sql.NullString `db:"commission"`
	ProductID      string         `db:"product_id"`
	XchID          int            `db:"xch_id"`
	MarketCategory string         `db:"market_category"`
}

type OrderFill struct {
	Orders []Order
	Fills  []Fill
}

func (c *Candle) UnmarshalJSON(data []byte) error {
	var temp struct {
		Timestamp string `json:"start"`
		Open      string `json:"open"`
		High      string `json:"high"`
		Low       string `json:"low"`
		Close     string `json:"close"`
		Volume    string `json:"volume"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	timestamp, err := strconv.ParseInt(temp.Timestamp, 10, 64)
	if err != nil {
		return err
	}

	open, err := strconv.ParseFloat(temp.Open, 64)
	if err != nil {
		return err
	}

	high, err := strconv.ParseFloat(temp.High, 64)
	if err != nil {
		return err
	}

	low, err := strconv.ParseFloat(temp.Low, 64)
	if err != nil {
		return err
	}

	close, err := strconv.ParseFloat(temp.Close, 64)
	if err != nil {
		return err
	}

	volume, err := strconv.ParseFloat(temp.Volume, 64)
	if err != nil {
		return err
	}

	c.Timestamp = timestamp
	c.Open = open
	c.High = high
	c.Low = low
	c.Close = close
	c.Volume = volume

	return nil
}
