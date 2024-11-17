package model

import (
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
	Portfolio         []Asset
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
	Asset            string  `json:"asset"`
	AvailableBalance Balance `json:"available_balance"`
	Hold             Balance `json:"hold_balance`
	Value            float64 `json:"value`
	XchID            int     `json:"xch_id"`
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
	Timestamp      string  `db:"time"`
	OrderID        string  `db:"order_id"`   // Exchange specific order identifier
	ProductID      string  `db:"product_id"` // xbt_usd_15
	TradeType      string  `db:"trade_type"` // Long / Short
	Side           string  `db:"side"`       // buy / sell
	XchID          int     `db:"xch_id"`
	MarketCategory string  `db:"market_category"` // (crypto / equities)_(spot / futures)
	Price          float64 `db:"price"`           // instrument_currency
	Size           float64 `db:"size"`            // How many of instrument
	FilledSize     float64 `db:"filled_size"`
	Status         string  `db:"status"`
	TotalFees      float64 `json:"total_fees"`
}

type OrderConfiguration struct {
	MarketMarketIoc *struct {
		QuoteSize string `json:"quote_size"`
		BaseSize  string `json:"base_size"`
	} `json:"market_market_ioc"`

	SorLimitIoc *struct {
		BaseSize   string `json:"base_size"`
		LimitPrice string `json:"limit_price"`
	} `json:"sor_limit_ioc"`

	LimitLimitGtc *struct {
		BaseSize   string `json:"base_size"`
		LimitPrice string `json:"limit_price"`
		PostOnly   bool   `json:"post_only"`
	} `json:"limit_limit_gtc"`

	LimitLimitGtd *struct {
		BaseSize   string `json:"base_size"`
		LimitPrice string `json:"limit_price"`
		EndTime    string `json:"end_time"`
		PostOnly   bool   `json:"post_only"`
	} `json:"limit_limit_gtd"`

	LimitLimitFok *struct {
		BaseSize   string `json:"base_size"`
		LimitPrice string `json:"limit_price"`
	} `json:"limit_limit_fok"`

	StopLimitStopLimitGtc *struct {
		BaseSize      string `json:"base_size"`
		LimitPrice    string `json:"limit_price"`
		StopPrice     string `json:"stop_price"`
		StopDirection string `json:"stop_direction"`
	} `json:"stop_limit_stop_limit_gtc"`

	StopLimitStopLimitGtd *struct {
		BaseSize      string `json:"base_size"`
		LimitPrice    string `json:"limit_price"`
		StopPrice     string `json:"stop_price"`
		EndTime       string `json:"end_time"`
		StopDirection string `json:"stop_direction"`
	} `json:"stop_limit_stop_limit_gtd"`

	TriggerBracketGtc *struct {
		BaseSize         string `json:"base_size"`
		LimitPrice       string `json:"limit_price"`
		StopTriggerPrice string `json:"stop_trigger_price"`
	} `json:"trigger_bracket_gtc"`

	TriggerBracketGtd *struct {
		BaseSize         string `json:"base_size"`
		LimitPrice       string `json:"limit_price"`
		StopTriggerPrice string `json:"stop_trigger_price"`
		EndTime          string `json:"end_time"`
	} `json:"trigger_bracket_gtd"`
}

type Fill struct {
	EntryID        string  `db:"entry_id" json:"entryid"`
	TradeID        string  `db:"trade_id" json:"tradeid"`
	OrderID        string  `db:"order_id" json:"orderid"`
	Timestamp      string  `db:"time" json:"time"`
	TradeType      string  `db:"trade_type" json:"tradetype"`
	Price          float64 `db:"price" json:"price,omitempty"`
	Size           float64 `db:"size" json:"size,string,omitempty"`
	Side           string  `db:"side" json:"side"`
	Commission     float64 `db:"commission" json:"commission,string,omitempty"`
	ProductID      string  `db:"product_id" json:"productid"`
	XchID          int     `db:"xch_id" json:"xch_id"`
	MarketCategory string  `db:"market_category" json:"marketcategory"`
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
