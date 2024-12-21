package db

import (
	"backend/alerts"
	"database/sql"
	_ "hm/alerts"
	"time"
)

func GetActiveAlerts(db *sql.DB) ([]alerts.Alert, error) {
	query := `
        SELECT id, product_id, type, price, status, xch_id, created_at, updated_at
        FROM alerts
        WHERE status = 'active'
    `
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []alerts.Alert
	for rows.Next() {
		var alert alerts.Alert
		err := rows.Scan(
			&alert.ID,
			&alert.ProductID,
			&alert.Type,
			&alert.Price,
			&alert.Status,
			&alert.XchID,
			&alert.CreatedAt,
			&alert.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, alert)
	}
	return alerts, nil
}

func UpdateAlertStatus(db *sql.DB, alertID int, status string) error {
	query := `
        UPDATE alerts
        SET status = $1, updated_at = $2
        WHERE id = $3
    `
	_, err := db.Exec(query, status, time.Now(), alertID)
	return err
}
