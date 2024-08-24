package common

import (
	"fmt"
)

type Candle struct {
    Timestamp   int64
    Open   float64
    High   float64
    Low    float64
    Close  float64
    Volume float64
}

type Timeframe struct {
    Label string
    Xch   string
    Tf    int
}

type Watchlist struct {
	Product		string
	Exchange	string
}

