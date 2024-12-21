package alerts

func NewAlertManager() *AlertManager {
	return &AlertManager{
		alerts: make(map[string][]Alert),
	}
}

func (am *AlertManager) UpdateAlerts(alerts []Alert) {
	am.alertMutex.Lock()
	defer am.alertMutex.Unlock()

	// Group alerts by product
	newAlerts := make(map[string][]Alert)
	for _, alert := range alerts {
		newAlerts[alert.ProductID] = append(newAlerts[alert.ProductID], alert)
	}
	am.alerts = newAlerts
}

func (am *AlertManager) ProcessTickerUpdate(productID string, price float64) []Alert {
	am.alertMutex.RLock()
	defer am.alertMutex.RUnlock()

	var triggeredAlerts []Alert
	if alerts, exists := am.alerts[productID]; exists {
		for _, alert := range alerts {
			if alert.Status != "active" {
				continue
			}

			triggered := false
			switch alert.Type {
			case "price_above":
				triggered = price > alert.Price
			case "price_below":
				triggered = price < alert.Price
			}

			if triggered {
				alert.Status = "triggered"
				triggeredAlerts = append(triggeredAlerts, alert)
			}

		}
	}
	return triggeredAlerts
}
