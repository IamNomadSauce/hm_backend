package model

import "time"

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
	return orders, nil
}

// Exchange operation
func (api *CoinbaseAPI) FetchCandles(product string, timeframe string, start, end time.Time) ([]Candle, error) {
	var candles []Candle
	return candles, nil
}

// Exchange operation
func (api *CoinbaseAPI) FetchFills() ([]Fill, error) {
	var fills []Fill
	return fills, nil
}

// Exchange operation
func (api *CoinbaseAPI) FetchPortfolio() ([]Asset, error) {
	var portfolio []Asset
	return portfolio, nil
}

// Database operation
func (api *CoinbaseAPI) FetchWatchlist() ([]Product, error) {
	var products []Product
	return products, nil
}

// Database operation
func (api *CoinbaseAPI) FetchTimeframes() ([]Timeframe, error) {
	var timeframes []Timeframe
	return timeframes, nil
}
