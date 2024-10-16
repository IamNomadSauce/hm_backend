package model

type CoinbaseExch struct {
	ID int
	Name string
	Timeframes []Timeframe
	Orders []Order
	Fills []Fill
	Watchlist []Product
	Portfolio []Asset
)

func (c *CoinbaseExchange) GetOrders() ([]Order, error) {
	var orders []Order
	return orders, nil
}

func (c *CoinbaseExchange) GetCandles(symbol, tf string) ([]Candle, error) {
	var candles []Candle
	return candles, nil
}

func (c *CoinbaseExchange) GetFills() ([]Fill, error) {
	var fills []Fill
	return fills, nil
}

func (c *CoinbaseExchange) GetPortfolio() ([]Asset, error) {
	var assets []Asset
	return assets, nil
}

func (c *CoinbaseExchange) GetWatchlist() ([]Product, error) {
	var product []Product
	return product, nil
}

func (c *CoinbaseExchange) GetTimeframes() ([]Timeframe, error) {
	var timeframes []Timeframe
	return timeframes, nil
}
