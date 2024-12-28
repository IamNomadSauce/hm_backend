package alerts

import "sync"

type Alert struct {
	ID        int     `json:"id"`
	ProductID string  `json:"product_id"`
	Type      string  `json:"type"`
	Price     float64 `json:"price"`
	Status    string  `json:"status"`
	XchID     int     `json:"xch_id"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type AlertManager struct {
	alerts     map[string][]Alert // map[productID][]Alert
	alertMutex sync.RWMutex
}
