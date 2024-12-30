package alerts

// func NewAlertManager() *AlertManager {
// 	return &AlertManager{
// 		alerts: make(map[string][]Alert),
// 	}
// }

// func (am *AlertManager) UpdateAlerts(alerts []Alert) {
// 	am.alertMutex.Lock()
// 	defer am.alertMutex.Unlock()

// 	// Group alerts by product
// 	newAlerts := make(map[string][]Alert)
// 	for _, alert := range alerts {
// 		newAlerts[alert.ProductID] = append(newAlerts[alert.ProductID], alert)
// 	}
// 	am.alerts = newAlerts
// }

// func (am *AlertManager) ProcessPriceAlerts(productID string, price float64) []Alert {
// 	am.alertMutex.RLock()
// 	defer am.alertMutex.RUnlock()

// 	var triggeredAlerts []Alert

// 	if alerts, exists := am.alerts[productID]; exists {
// 		for _, alert := range alerts {
// 			if alert.Status != "active" {
// 				continue
// 			}

// 			triggered := false
// 			switch alert.Condition {
// 			case "wick_above":
// 				if price > alert.Price {
// 					triggered = true
// 				}

// 			case "wick_below":
// 				if price < alert.Price {
// 					triggered = true
// 				}
// 			}

// 			if triggered {
// 				alert.Status = "triggered"
// 				triggeredAlerts = append(triggeredAlerts, alert)
// 			}
// 		}
// 	}
// 	return triggeredAlerts
// }

// func (am *AlertManager) ProcessTickerUpdate(productID string, price float64) []Alert {
// 	am.alertMutex.RLock()
// 	defer am.alertMutex.RUnlock()

// 	var triggeredAlerts []Alert
// 	if alerts, exists := am.alerts[productID]; exists {
// 		for _, alert := range alerts {
// 			if alert.Status != "active" {
// 				continue
// 			}

// 			triggered := false
// 			switch alert.Condition {
// 			case "wick_above":
// 				triggered = price > alert.Price
// 			case "wick_below":
// 				triggered = price < alert.Price
// 			case "close_above":
// 			case "close_below":
// 			}

// 			if triggered {
// 				alert.Status = "triggered"
// 				triggeredAlerts = append(triggeredAlerts, alert)
// 			}

// 		}
// 	}
// 	return triggeredAlerts
// }

// func (am *AlertManager) ListenForChanges(database *sql.DB) error {
// 	listener := pq.NewListener(
// 		"connection_string", // This needs to be altered
// 		0,
// 		time.Minute,
// 		func(ev pq.ListenerEventType, err error) {
// 			if err != nil {
// 				log.Printf("Error in database listener: %v", err)
// 			}
// 		},
// 	)

// 	err := listener.Listen("table_changes")
// 	if err != nil {
// 		return fmt.Errorf("error setting up database listener; %v", err)
// 	}

// 	go func() {
// 		for notification := range listener.Notify {
// 			var change struct {
// 				Table     string          `json:"table"`
// 				Operation string          `json:"operation"`
// 				Data      json.RawMessage `json:"data"`
// 			}

// 			if err := json.Unmarshal([]byte(notification.Extra), &change); err != nil {
// 				log.Printf("Error parsing notification: %v", err)
// 				continue
// 			}

// 			switch change.Table {
// 			case "alerts":
// 				alerts, err := db.GetAlerts(database, 1, "active")
// 				if err != nil {
// 					log.Printf("Error refreshing alerts: %v", err)
// 					continue
// 				}
// 				am.UpdateAlerts(alerts)
// 				// case "candles":
// 				// 	var candle model.Candle
// 				// 	if err := json.Unmarshal(change.Data, &candle); err != nil {
// 				// 		continue
// 				// 	}
// 				// 	am.ProcessCandleAlert(candle.ProductID, candle.Timeframe, candle)

// 			}
// 		}
// 	}()

// 	return nil
// }

// func (am *AlertManager) LIstenForChanges(db *sql.DB) error {
// 	listener := pq.NewListener(db.DriverName(), 10*)
// }
