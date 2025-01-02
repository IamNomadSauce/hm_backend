package model

import (
	"backend/common"
	"time"
)

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
	Trades            []Trade
	Triggers          []common.Trigger
}

type ExchangeAPI interface {
	FetchOrdersFills() ([]Order, error)
	FetchFills() ([]Fill, error)
	FetchPortfolio() ([]Asset, error)
	FetchCandles(product string, timeframe Timeframe, start, end time.Time) ([]common.Candle, error)
	FetchAvailableProducts() ([]Product, error)
	PlaceBracketOrder(trade_group Trade) error
	PlaceOrder(trade Trade) (string, error)
	// ConnectWebSocket() error
	ConnectUserWebsocket() error
	ConnectMarketDataWebSocket() error
	GetOrder(orderID string) (*Order, error)
	// ProcessPrice(productID string, price float64)
	// ProcessCandle(productID string, timeframe string, candle common.Candle)
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
	ID               int     `json:"id"`
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

type Trade struct {
	ID           int       `json:"id"`
	GroupID      string    `json:"group_id"`
	ProductID    string    `json:"product_id"`
	Side         string    `json:"side"`
	EntryPrice   float64   `json:"entry_price"`
	StopPrice    float64   `json:"stop_price"`
	Size         float64   `json:"size"`
	EntryOrderID string    `json:"entry_order_id"`
	StopOrderID  string    `json:"stop_order_id"`
	EntryStatus  string    `json:"entry_status"`
	StopStatus   string    `json:"stop_status"`
	PTPrice      float64   `json:"pt_price"`
	PTStatus     string    `json:"pt_status"`
	PTOrderID    string    `json:"pt_order_id"`
	PTAmount     int       `json:"pt_amount"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	XchID        int       `json:"xch_id"`
}

type TradeBlock struct {
	ProductID     string    `json:"product_id"`
	GroupID       string    `json:"group_id"`
	Side          string    `json:"side"`
	Size          float64   `json:"size"`
	EntryPrice    float64   `json:"entry_price"`
	StopPrice     float64   `json:"stop_price"`
	ProfitTargets []float64 `json:"profit_targets"`
	XchID         int       `json:"xch_id"`
	// Trigger on Alert?
}

// type Alert struct {
// 	ID             string    `json:"id"`
// 	ProductID      string    `json:"product_id"`
// 	Price          float64   `json:"price"`
// 	TriggerOnClose bool      `json:"clolseorwick"`
// 	AboveBelow     bool      `json:"abovebelow"`
// 	TimeFrame      Timeframe `json:"timeframe"`
// 	Type           string    `json:"type"`
// 	// TradeBlock GroupID?
// }

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
	ProductID      string  `db:"product_id" json:"product_id"`
	XchID          int     `db:"xch_id" json:"xch_id"`
	MarketCategory string  `db:"market_category" json:"market_category"`
}

type OrderFill struct {
	Orders []Order
	Fills  []Fill
}
