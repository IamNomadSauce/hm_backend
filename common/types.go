package common

import (
	"encoding/json"
	"strconv"
)

type Candle struct {
	ProductID string
	Timestamp int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

type Trigger struct {
	ID             int     `json:"id"`
	ProductID      string  `json:"product_id"`
	Type           string  `json:"type"` // Close or Wick
	Price          float64 `json:"price"`
	Timeframe      string  `json:"timeframe"`
	CandleCount    int     `json:"candle_count"`
	Condition      string  `json:"condition"`
	Status         string  `json:"status"` // Maybe should be bool?
	TriggeredCount int     `json:"triggered_count"`
	XchID          int     `json:"xch_id"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	Action         string  `json:"action"`
}

// type Trendline struct {
// 	ID         int
// 	StartTime  int64
// 	StartPrice float64
// 	StartInv float64
// 	EndTime    int64
// 	EndPrice   float64
// 	EndInv float64
// 	Direction  string
// 	Done       string
// }

// Trendline represents a trendline with start and end points, type, and status.
type Trendline struct {
	Start     Point  `json:"start"`
	End       Point  `json:"end"`
	Direction string `json:"type"`   // "up" or "down"
	Status    string `json:"status"` // "current" or "done"
}

// Point represents a point in the trendline with time, price, inverse price, and trend start price.
type Point struct {
	Time       int64   `json:"time"`
	Point      float64 `json:"point"`
	Inv        float64 `json:"inv"`
	TrendStart float64 `json:"trendStart"`
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
