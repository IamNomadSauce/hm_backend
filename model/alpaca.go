package model

type AlpacaExchange struct {
	ID int
	Name string
	Timeframes []Timeframe
	Orders []Order
	Fills []Fill
	Watchlist []Product
	Portfolio []Asset
)

func (c *AlpacaExchange) GetOrders() ([]Order, error) {
	var orders []Order
	return orders, nil
}

func (c *AlpacaExchange) GetCandles(symbol, tf string) ([]Candle, error) {
	var candles []Candle
	return candles, nil
}

func (c *AlpacaExchange) GetFills() ([]Fill, error) {
	var fills []Fill
	return fills, nil
}

func (c *AlpacaExchange) GetPortfolio() ([]Asset, error) {
	var assets []Asset
	return assets, nil
}

func (c *AlpacaExchange) GetWatchlist() ([]Product, error) {
	var product []Product
	return product, nil
}

func (c *AlpacaExchange) GetTimeframes() ([]Timeframe, error) {
	var timeframes []Timeframe
	return timeframes, nil
}
