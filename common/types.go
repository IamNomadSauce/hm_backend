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
