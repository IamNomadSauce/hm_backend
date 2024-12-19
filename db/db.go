package db

import (
	"backend/model"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time"

	"github.com/google/uuid"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var host string
var port int
var user string
var password string
var dbname string

// var DB *sql.DB

func DBConnect() (*sql.DB, error) {
	log.Println("\n------------------------------\n DBConnect \n------------------------------\n")
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v\n", err)
		// Continue execution, as environment variables might be set elsewhere
	}

	host = os.Getenv("PG_HOST")
	portStr := os.Getenv("PG_PORT")
	user = os.Getenv("PG_USER")
	password = os.Getenv("PG_PASS")
	dbname = os.Getenv("PG_DBNAME")

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("Invalid port number: %v", err)
	}

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	// log.Printf("Attempting to connect with: host=%s port=%d user=%s dbname=%s\n", host, port, user, dbname)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, fmt.Errorf("Error opening Postgres connection: %v", err)
	}

	// Set connection pool parameters
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Ping the database to verify the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("Error pinging database: %v", err)
	}

	log.Println("Successfully connected to database")
	return db, nil
}

func CreateTables(db *sql.DB) error {
	// log.Println("\n------------------------------\n CreateTables \n------------------------------\n")

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS exchanges (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL
		);
	`)
	if err != nil {
		log.Println("Failed to create exchanges table: ", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS timeframes (
			id SERIAL PRIMARY KEY,
			xch_id INTEGER REFERENCES exchanges(id),
			tf VARCHAR(10) NOT NULL,
			minutes INTEGER NOT NULL,
			endpoint VARCHAR(50) NOT NULL
		);
	`)
	if err != nil {
		log.Println("Failed to create timeframes table: ", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			order_id VARCHAR(255) UNIQUE NOT NULL, 
			product_id VARCHAR(255) NOT NULL, 
			trade_type VARCHAR(255) NOT NULL, 
			side VARCHAR(25) NOT NULL, 
			price NUMERIC NOT NULL, 
			size NUMERIC,
			xch_id INTEGER REFERENCES exchanges(id),
			market_category varchar(25) NOT NULL, 
			time BIGINT NOT NULL,
			total_fees VARCHAR(25)
		);
		
	`)
	if err != nil {
		log.Println("Failed to create orders table: ", err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS fills (
			entry_id VARCHAR(255) PRIMARY KEY,
			trade_id VARCHAR(255),
			order_id VARCHAR(255),
			time BIGINT,
			trade_type VARCHAR(50),
			price NUMERIC(20,8),
			size NUMERIC(20,8),
			side VARCHAR(50),
			commission NUMERIC(20,8),
			product_id VARCHAR(255),
			xch_id INTEGER,
			market_category VARCHAR(50)
		);
	`)
	if err != nil {
		log.Println("Failed to create fills table: ", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS watchlist (
			id SERIAL PRIMARY KEY,
			xch_id INTEGER REFERENCES exchanges(id),
			product VARCHAR(50) NOT NULL,
        CONSTRAINT unique_xch_product UNIQUE (xch_id, product)
    	);
	`)
	if err != nil {
		log.Println("Failed to create watchlist table: ", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS available_products (
			id SERIAL PRIMARY KEY,
			xch_id INTEGER REFERENCES exchanges(id),
			product_id VARCHAR(50) NOT NULL,
			base_name VARCHAR(50) NOT NULL,
			quote_name VARCHAR(50) NOT NULL,
			status VARCHAR(50) NOT NULL,
			price NUMERIC NOT NULL,
			volume NUMERIC NOT NULL,
			base_currency_id VARCHAR(50) NOT NULL,
			quote_currency_id VARCHAR(50) NOT NULL,
			UNIQUE(xch_id, product_id)  
		);
	`)
	if err != nil {
		log.Printf("Failed to create available_products: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS portfolio (
			id SERIAL PRIMARY KEY,
			asset VARCHAR(255),
			available_balance_value VARCHAR(255),
			available_balance_currency VARCHAR(255),
			hold_balance_value VARCHAR(255),
			hold_balance_currency VARCHAR(255),
			value FLOAT8,
			xch_id INTEGER
		);
	`)
	if err != nil {
		log.Printf("Failed to create portfolios: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS trades (
			id SERIAL PRIMARY KEY,
			group_id VARCHAR(255),
			product_id VARCHAR(255),
			side VARCHAR(10),
			entry_price FLOAT,
			stop_price FLOAT,
			size FLOAT,
			entry_order_id VARCHAR(255),
			stop_order_id VARCHAR(255),
			entry_status VARCHAR(20),
			stop_status VARCHAR(20),
			pt_price FLOAT,
			pt_status VARCHAR(20),
			pt_order_id VARCHAR(255),
			pt_amount INT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			xch_id INTEGER
		);

	`)
	if err != nil {
		log.Printf("Failed to create trades: %v", err)
	}

	return nil
}

func Get_Available_Products(exchange model.Exchange, db *sql.DB) ([]model.Product, error) {
	query := `
		SELECT 
			id, 
			xch_id, 
			product_id,
			base_name,
			quote_name,
			status, 
			price,
			volume,
			base_currency_id,
			quote_currency_id
		FROM available_products 
		WHERE xch_id = $1
		ORDER BY volume DESC
	`
	rows, err := db.Query(query, exchange.ID)
	if err != nil {
		return nil, fmt.Errorf("Error getting all available products for xch: %s,\n%w", exchange, err)
	}

	defer rows.Close()

	var products []model.Product

	for rows.Next() {
		var product model.Product
		err := rows.Scan(
			&product.ID,
			&product.XchID,
			&product.ProductID,
			&product.BaseName,
			&product.QuoteName,
			&product.Status,
			&product.Price,
			&product.Volume_24h,
			&product.Base_Currency_ID,
			&product.Quote_Currency_ID,
		)
		if err != nil {
			return nil, fmt.Errorf("Error scanning rows for available_product %s\n%w", exchange, err)
		}
		products = append(products, product)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("Error iterating rows for available_product: %s\n%w", exchange, err)
	}
	return products, nil
}

func Write_AvailableProducts(exchange int, products []model.Product, db *sql.DB) error {
	// log.Println("Write_AvailableProducts", exchange, len(products))
	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if not committed

	// Prepare the statement
	stmt, err := tx.Prepare(`
        INSERT INTO available_products (
            xch_id,
            product_id,
            base_name,
            quote_name,
            status,
            price,
            volume,
            base_currency_id,
            quote_currency_id
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        ON CONFLICT (xch_id, product_id) DO UPDATE SET
            base_name = EXCLUDED.base_name,
            quote_name = EXCLUDED.quote_name,
            status = EXCLUDED.status,
            price = EXCLUDED.price,
            volume = EXCLUDED.volume,
            base_currency_id = EXCLUDED.base_currency_id,
            quote_currency_id = EXCLUDED.quote_currency_id
    `)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Execute for each product
	for _, product := range products {
		_, err := stmt.Exec(
			exchange,
			product.ProductID,
			product.BaseName,
			product.QuoteName,
			product.Status,
			product.Price,
			product.Volume_24h,
			product.Base_Currency_ID,
			product.Quote_Currency_ID,
		)
		if err != nil {
			return fmt.Errorf("failed to execute statement for product %s: %w", product.ProductID, err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func ListTables(db *sql.DB) error {
	// log.Println("\n------------------------------\n ListTables \n------------------------------\n")
	rows, err := db.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'")
	if err != nil {
		log.Println("Error listing tables", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			log.Println("Error scanning table name", err)
			return err
		}
		// log.Println(" -", tableName)
	}

	return nil
}

func Write_Orders(xch_id int, orders []model.Order, db *sql.DB) error { // Current Orders for all accounts
	// log.Println("\n------------------------------\n Write Order \n------------------------------\n")

	_, err := db.Exec("DELETE FROM orders WHERE xch_id = $1;", xch_id)
	if err != nil {
		fmt.Sprintf("Failed to delete existing orders: \n%v", err)
		return err
	}

	// log.Println("Orders Deleted")

	insertQuery := `
	INSERT INTO orders (order_id, product_id, trade_type, side, price, size, xch_id, market_category, time, total_fees)
	VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9, $10);
	`
	for _, order := range orders {
		_, err := db.Exec(insertQuery, order.OrderID, order.ProductID, order.TradeType, order.Side, order.Price, order.Size, xch_id, order.MarketCategory, order.Timestamp, order.TotalFees)
		if err != nil {
			log.Printf("Error inserting into Order table: \n%v", err)
			return err
		}

	}
	// log.Println(orders)
	// log.Println(len(orders), "orders added to db")
	return nil
}

// Helper function to convert string to sql.NullFloat64
func toNullFloat64(s string) sql.NullFloat64 {
	if s == "" {
		return sql.NullFloat64{Valid: false}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: f, Valid: true}
}

func convertOrderToFill(order model.Order) model.Fill {
	return model.Fill{
		Timestamp:      order.Timestamp,
		EntryID:        fmt.Sprintf("%s-%d", order.OrderID, order.Timestamp),
		TradeID:        order.OrderID,
		OrderID:        order.OrderID,
		TradeType:      order.TradeType,
		Price:          order.Price,
		Size:           order.Size,
		Side:           order.Side,
		Commission:     order.TotalFees,
		ProductID:      order.ProductID,
		XchID:          order.XchID,
		MarketCategory: order.MarketCategory,
	}
}

func Write_Fills(xch_id int, fills []model.Fill, db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	insertQuery := `
        INSERT INTO fills (
            entry_id, trade_id, order_id, time, trade_type,
            price, size, side, commission, product_id,
            xch_id, market_category
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
        ON CONFLICT (entry_id) DO UPDATE SET
            trade_id = EXCLUDED.trade_id,
            order_id = EXCLUDED.order_id,
            time = EXCLUDED.time,
            trade_type = EXCLUDED.trade_type,
            price = EXCLUDED.price,
            size = EXCLUDED.size,
            side = EXCLUDED.side,
            commission = EXCLUDED.commission,
            product_id = EXCLUDED.product_id,
            xch_id = EXCLUDED.xch_id,
            market_category = EXCLUDED.market_category
    `

	stmt, err := tx.Prepare(insertQuery)
	if err != nil {
		return fmt.Errorf("error preparing statement: %w", err)
	}
	defer stmt.Close()

	for _, fill := range fills {
		_, err := stmt.Exec(
			fill.EntryID,
			fill.TradeID,
			fill.OrderID,
			fill.Timestamp,
			fill.TradeType,
			fill.Price,
			fill.Size,
			fill.Side,
			fill.Commission,
			fill.ProductID,
			fill.XchID,
			fill.MarketCategory,
		)
		if err != nil {
			return fmt.Errorf("error inserting fill: %w", err)
		}
	}
	// log.Println(len(fills), "fills added to db")

	return tx.Commit()
}

// Helper function to validate numeric fields
func validateNumericField(fieldName, value string) error {
	if value == "" {
		return nil // Empty strings will be converted to NULL
	}
	_, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("invalid %s value '%s': %v", fieldName, value, err)
	}
	return nil
}

// Helper function to convert empty strings to "0"
func convertEmptyToZero(value string) string {
	if value == "" {
		return "0"
	}
	return value
}

// ---------------------------------------------------------------

func Write_Candles(candles []model.Candle, product, exchange, tf string, db *sql.DB) error {
	// log.Println("\n------------------------------\n Write Candles \n------------------------------\n")
	// log.Println(product, tf, exchange, len(candles))

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("Failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	product = strings.Replace(product, "-", "_", -1)

	_, err = tx.Exec(fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS %s_%s_%s (
            timestamp BIGINT PRIMARY KEY,
            open    FLOAT NOT NULL,
            high    FLOAT NOT NULL,
            low     FLOAT NOT NULL,
            close   FLOAT NOT NULL,
            volume  FLOAT NOT NULL
        );
    `, product, tf, exchange))
	if err != nil {
		return fmt.Errorf("Failed to create candles table: %w", err)
	}

	stmt, err := tx.Prepare(fmt.Sprintf(`
        INSERT INTO %s_%s_%s (timestamp, open, high, low, close, volume)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (timestamp) DO UPDATE SET
            open = EXCLUDED.open,
            high = EXCLUDED.high,
            low = EXCLUDED.low,
            close = EXCLUDED.close,
            volume = EXCLUDED.volume
    `, product, tf, exchange))
	if err != nil {
		return fmt.Errorf("Failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	successCount := 0
	for _, candle := range candles {
		_, err := stmt.Exec(candle.Timestamp, candle.Open, candle.High, candle.Low, candle.Close, candle.Volume)
		if err != nil {
			return fmt.Errorf("Failed to insert candle: %w", err)
		}
		successCount++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("Failed to commit transaction: %w", err)
	}

	// log.Printf("Total candles inserted or updated: %d\n", successCount)
	return nil
}

func Write_Watchlist(db *sql.DB, exchangeID int, productID string) error {
	// First verify the exchange exists
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM exchanges WHERE id = $1)", exchangeID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error checking exchange existence: %v", err)
	}
	if !exists {
		return fmt.Errorf("exchange with ID %d does not exist", exchangeID)
	}

	// Then proceed with insert
	_, err = db.Exec(`
        INSERT INTO watchlist (xch_id, product)
        VALUES ($1, $2)
        ON CONFLICT (xch_id, product) DO NOTHING
    `, exchangeID, productID)

	if err != nil {
		return fmt.Errorf("error writing to watchlist: %v", err)
	}
	return nil
}

func Get_Exchange(id int, db *sql.DB) (model.Exchange, error) {
	// log.Printf("\n-------------------------------------\n Get Exchange  %v\n-------------------------------------\n", id)

	var exchange model.Exchange
	var err error

	xch_row := db.QueryRow("SELECT id, name FROM exchanges WHERE id = $1", id)
	if err := xch_row.Scan(&exchange.ID, &exchange.Name); err != nil {
		return exchange, fmt.Errorf("error scanning exchange: %w", err)
	}

	exchange.Timeframes, err = Get_Timeframes(exchange.ID, db)
	if err != nil {
		return exchange, fmt.Errorf("error getting timeframes: %w", err)
	}
	exchange.Orders, err = Get_Orders(exchange.ID, db)
	if err != nil {
		return exchange, fmt.Errorf("error getting orders: %w", err)
	}
	exchange.Fills, err = Get_Fills(exchange.ID, db)
	if err != nil {
		return exchange, fmt.Errorf("error getting fills: %w", err)
	}
	exchange.Watchlist, err = Get_Watchlist(exchange, db)
	if err != nil {
		return exchange, fmt.Errorf("error getting watchlist: %w", err)
	}
	exchange.AvailableProducts, err = Get_Available_Products(exchange, db)
	if err != nil {
		return exchange, fmt.Errorf("error getting available_products: %w", err)
	}
	exchange.Portfolio, err = Get_Portfolio(exchange.ID, db)
	if err != nil {
		return exchange, fmt.Errorf("error getting portfolio: %w", err)
	}
	exchange.Trades, err = GetTradesByExchange(db, exchange.ID)
	if err != nil {
		return exchange, fmt.Errorf("error getting trades: %v", err)
	}

	switch exchange.Name {
	case "Coinbase":
		exchange.API = &model.CoinbaseAPI{
			APIKey:      os.Getenv("CBAPIKEY"),
			APISecret:   os.Getenv("CBAPISECRET"),
			BaseURL:     "https://api.coinbase.com",
			CandleLimit: 350,
			ExchangeID:  exchange.ID,
		}
		exchange.CandleLimit = 350
	case "Alpaca":
	}

	return exchange, nil
}

func Get_Exchanges(db *sql.DB) ([]model.Exchange, error) {
	// log.Printf("\n-------------------------------------\n Get All Exchanges 10.25.24 \n-------------------------------------\n")

	var exchanges []model.Exchange

	rows, err := db.Query("SELECT * FROM exchanges")
	if err != nil {
		return nil, fmt.Errorf("Error getting exchanges: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var exchange model.Exchange

		if err := rows.Scan(&exchange.ID, &exchange.Name); err != nil {
			return nil, fmt.Errorf("Error scanning exchange: %w", err)
		}

		exchange, err := Get_Exchange(exchange.ID, db)
		if err != nil {
			return nil, fmt.Errorf("error getting exchange: %w", err)
		}

		exchanges = append(exchanges, exchange)
	}

	return exchanges, nil
}
func Get_Portfolio(id int, db *sql.DB) ([]model.Asset, error) {
	var portfolio []model.Asset

	rows, err := db.Query(`
        SELECT id, asset, 
               available_balance_value, available_balance_currency,
               hold_balance_value, hold_balance_currency,
               value, xch_id 
        FROM portfolio 
        WHERE xch_id = $1;`, id)
	if err != nil {
		return nil, fmt.Errorf("Error retrieving assets from %d portfolio: %w", id, err)
	}
	defer rows.Close()

	for rows.Next() {
		var asset model.Asset
		var availBalValue, availBalCurrency, holdBalValue, holdBalCurrency string

		if err := rows.Scan(
			&asset.ID,
			&asset.Asset,
			&availBalValue,
			&availBalCurrency,
			&holdBalValue,
			&holdBalCurrency,
			&asset.Value,
			&asset.XchID,
		); err != nil {
			return nil, fmt.Errorf("Error scanning portfolio: %w", err)
		}

		// Create Balance structs
		asset.AvailableBalance = model.Balance{
			Value:    availBalValue,
			Currency: availBalCurrency,
		}
		asset.Hold = model.Balance{
			Value:    holdBalValue,
			Currency: holdBalCurrency,
		}

		portfolio = append(portfolio, asset)
	}

	return portfolio, nil
}

func Write_Portfolio(xch_id int, portfolio []model.Asset, db *sql.DB) error {
	_, err := db.Exec("DELETE FROM portfolio WHERE xch_id = $1;", xch_id)
	if err != nil {
		return fmt.Errorf("Failed to delete existing portfolio: %w", err)
	}

	insertQuery := `
        INSERT INTO portfolio (
            asset, 
            available_balance_value, available_balance_currency,
            hold_balance_value, hold_balance_currency,
            value, xch_id
        )
        VALUES($1, $2, $3, $4, $5, $6, $7);
    `

	for _, asset := range portfolio {
		_, err = db.Exec(insertQuery,
			asset.Asset,
			asset.AvailableBalance.Value,
			asset.AvailableBalance.Currency,
			asset.Hold.Value,
			asset.Hold.Currency,
			asset.Value,
			xch_id)
		if err != nil {
			return fmt.Errorf("Error inserting into portfolio table: %w", err)
		}
	}

	// log.Printf("%d assets added to portfolio", len(portfolio))
	return nil
}

func Get_Orders(id int, db *sql.DB) ([]model.Order, error) {
	fmt.Sprintf("\n-------------------------------------\n Get Orders  %v\n-------------------------------------\n", id)

	var orders []model.Order
	orderRows, err := db.Query("SELECT order_id, product_id, trade_type, side, price, size, xch_id, market_category, time FROM orders WHERE xch_id = $1", id)
	if err != nil {
		fmt.Sprintf("Error retrieving orders for %d \n%v", id, err)
	}

	defer orderRows.Close()

	for orderRows.Next() {
		var order model.Order

		if err := orderRows.Scan(&order.OrderID, &order.ProductID, &order.TradeType, &order.Side, &order.Price, &order.Size, &order.XchID, &order.MarketCategory, &order.Timestamp); err != nil {
			return orders, fmt.Errorf("Error scanning order %w", err)
		}
		orders = append(orders, order)
	}

	return orders, nil

}

func Get_Fills(xch_id int, database *sql.DB) ([]model.Fill, error) {
	query := `
        SELECT 
            entry_id, trade_id, order_id, time, trade_type,
            price, size, side, commission, product_id,
            xch_id, market_category
        FROM fills 
        WHERE xch_id = $1
		ORDER BY time DESC
    `

	rows, err := database.Query(query, xch_id)
	if err != nil {
		return nil, fmt.Errorf("Error querying fills: %w", err)
	}
	defer rows.Close()

	var fills []model.Fill
	for rows.Next() {
		var fill model.Fill
		err := rows.Scan(
			&fill.EntryID,
			&fill.TradeID,
			&fill.OrderID,
			&fill.Timestamp,
			&fill.TradeType,
			&fill.Price,
			&fill.Size,
			&fill.Side,
			&fill.Commission,
			&fill.ProductID,
			&fill.XchID,
			&fill.MarketCategory,
		)
		if err != nil {
			return nil, fmt.Errorf("Error scanning fill: %w", err)
		}
		fills = append(fills, fill)
	}

	return fills, nil
}

func Get_Watchlist(exchange model.Exchange, db *sql.DB) ([]model.Product, error) {
	// log.Printf("\n-------------------------------------\n Get Watchlist  %v\n-------------------------------------\n", exchange.Name)

	var watchlist []model.Product

	watchlistRows, err := db.Query("SELECT id, product, xch_id FROM watchlist WHERE xch_id = $1", exchange.ID)
	if err != nil {
		return watchlist, fmt.Errorf("Error retrieving watchlist: %w", err)
	}

	defer watchlistRows.Close()

	for watchlistRows.Next() {
		var ticker model.Product

		if err := watchlistRows.Scan(&ticker.ID, &ticker.ProductID, &ticker.XchID); err != nil {
			return watchlist, fmt.Errorf("Error scanning watchlist: %w", err)
		}
		watchlist = append(watchlist, ticker)

	}
	// log.Println("Watchlist Retrieved: ", watchlist)
	return watchlist, nil
}

func Get_Timeframes(id int, db *sql.DB) ([]model.Timeframe, error) {
	fmt.Sprintf("\n-------------------------------------\n Get Timeframes  %v\n-------------------------------------\n", id)

	var timeframes []model.Timeframe

	timeframe_rows, err := db.Query("SELECT id, xch_id, tf, minutes, endpoint FROM timeframes WHERE xch_id = $1", id)
	if err != nil {
		return timeframes, fmt.Errorf("Error querying timeframes: %w", err)
	}
	defer timeframe_rows.Close()

	for timeframe_rows.Next() {
		var tf model.Timeframe

		if err := timeframe_rows.Scan(&tf.ID, &tf.XchID, &tf.TF, &tf.Minutes, &tf.Endpoint); err != nil {
			return timeframes, fmt.Errorf("Error scanning timeframe: %w", id)
		}
		timeframes = append(timeframes, tf)
	}

	return timeframes, nil
}

func Get_Candles(product, tf, xch string, db *sql.DB) ([]model.Candle, error) {
	log.Printf("DB:Get Candles %s_%s_%s", product, tf, xch)

	//product = strings.Trim("-", "_")
	var candles []model.Candle
	log.Printf("DB:Get Candles2 %s_%s_%s", product, tf, xch)

	// Construct the table name
	tableName := fmt.Sprintf("%s_%s_%s", product, tf, xch)

	// Use parameterized query to prevent SQL injection
	query := fmt.Sprintf("SELECT timestamp, open, high, low, close, volume FROM %s ORDER BY timestamp DESC LIMIT 1000", tableName)

	candle_rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("Error querying candles: %w", err)
	}
	defer candle_rows.Close()

	for candle_rows.Next() {
		var candle model.Candle
		err := candle_rows.Scan(
			&candle.Timestamp,
			&candle.Open,
			&candle.High,
			&candle.Low,
			&candle.Close,
			&candle.Volume,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning candle row: %w", err)
		}
		candles = append(candles, candle)
	}

	if err = candle_rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating candle rows: %w", err)
	}

	return candles, nil
}

func Get_All_Candles(product, tf, xch string, db *sql.DB) ([]model.Candle, error) {
	log.Printf("DB:Get All Candles %s_%s_%s", product, tf, xch)

	var candles []model.Candle

	product = strings.Replace(product, "-", "_", -1)
	fmt.Println(product)

	// Construct the table name
	tableName := fmt.Sprintf("%s_%s_%s", product, tf, xch)

	// Use parameterized query to prevent SQL injection
	query := fmt.Sprintf("SELECT timestamp, open, high, low, close, volume FROM %s ORDER BY timestamp", tableName)

	candle_rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying candles: %w", err)
	}
	defer candle_rows.Close()

	for candle_rows.Next() {
		var candle model.Candle
		err := candle_rows.Scan(
			&candle.Timestamp,
			&candle.Open,
			&candle.High,
			&candle.Low,
			&candle.Close,
			&candle.Volume,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning candle row: %w", err)
		}
		candles = append(candles, candle)
	}

	if err = candle_rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating candle rows: %w", err)
	}

	return candles, nil
}

// --------------------------------------------------
// Trades
// --------------------------------------------------

// all trades in a trade-block
func WriteTrades(db *sql.DB, trades []model.Trade) error {
	groupID := uuid.New().String()

	for i, trade := range trades {
		trade.GroupID = groupID
		trade.PTAmount = i + 1

		query := `
            INSERT INTO trades (
                group_id, product_id, side, entry_price, stop_price, size,
                entry_order_id, stop_order_id, entry_status, stop_status,
                pt_price, pt_status, pt_order_id, pt_amount,
                created_at, updated_at, xch_id
            ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
            RETURNING id`

		err := db.QueryRow(
			query,
			trade.GroupID,
			trade.ProductID,
			trade.Side,
			trade.EntryPrice,
			trade.StopPrice,
			trade.Size,
			trade.EntryOrderID,
			trade.StopOrderID,
			trade.EntryStatus,
			trade.StopStatus,
			trade.PTPrice,
			trade.PTStatus,
			trade.PTOrderID,
			trade.PTAmount,
			time.Now(),
			time.Now(),
			trade.XchID,
		).Scan(&trade.ID)

		if err != nil {
			return err
		}
	}
	return nil
}

func GetAllTrades(db *sql.DB) ([]model.Trade, error) {
	query := `SELECT * FROM trades`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []model.Trade
	for rows.Next() {
		var t model.Trade
		err := rows.Scan(
			&t.ID,
			&t.GroupID,
			&t.ProductID,
			&t.Side,
			&t.EntryPrice,
			&t.StopPrice,
			&t.Size,
			&t.EntryOrderID,
			&t.StopOrderID,
			&t.EntryStatus,
			&t.StopStatus,
			&t.PTPrice,
			&t.PTStatus,
			&t.PTOrderID,
			&t.PTAmount,
			&t.CreatedAt,
			&t.UpdatedAt,
			&t.XchID,
		)
		if err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	return trades, nil
}

func GetTradesByExchange(db *sql.DB, exchange_id int) ([]model.Trade, error) {
	query := `SELECT * FROM trades WHERE xch_id = $1`
	rows, err := db.Query(query, exchange_id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []model.Trade
	for rows.Next() {
		var t model.Trade
		err := rows.Scan(
			&t.ID,
			&t.GroupID,
			&t.ProductID,
			&t.Side,
			&t.EntryPrice,
			&t.StopPrice,
			&t.Size,
			&t.EntryOrderID,
			&t.StopOrderID,
			&t.EntryStatus,
			&t.StopStatus,
			&t.PTPrice,
			&t.PTStatus,
			&t.PTOrderID,
			&t.PTAmount,
			&t.CreatedAt,
			&t.UpdatedAt,
			&t.XchID,
		)
		if err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	return trades, nil
}

func GetTradesByGroup(db *sql.DB, groupID string) ([]model.Trade, error) {
	query := `SELECT * FROM trades WHERE group_id = $1 ORDER BY pt_amount`
	rows, err := db.Query(query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []model.Trade
	for rows.Next() {
		var t model.Trade
		err := rows.Scan(
			&t.ID,
			&t.GroupID,
			&t.ProductID,
			&t.Side,
			&t.EntryPrice,
			&t.StopPrice,
			&t.Size,
			&t.EntryOrderID,
			&t.StopOrderID,
			&t.EntryStatus,
			&t.StopStatus,
			&t.PTPrice,
			&t.PTStatus,
			&t.PTOrderID,
			&t.PTAmount,
			&t.CreatedAt,
			&t.UpdatedAt,
			&t.XchID,
		)
		if err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	return trades, nil
}

// Add debug logging to UpdateTradeEntry
func UpdateTradeEntry(db *sql.DB, tradeID int, orderID string) error {
	log.Printf("Updating trade entry - Trade ID: %d, Order ID: %s", tradeID, orderID)

	query := `
        UPDATE trades 
        SET
            entry_order_id = $1,
            entry_status = 'PENDING', 
            updated_at = $2
        WHERE id = $3
        RETURNING entry_order_id, entry_status`

	var updatedOrderID, updatedStatus string
	err := db.QueryRow(query, orderID, time.Now(), tradeID).Scan(&updatedOrderID, &updatedStatus)
	if err != nil {
		return fmt.Errorf("failed to update trade entry: %w", err)
	}

	log.Printf("Trade entry updated - Order ID: %s, Status: %s", updatedOrderID, updatedStatus)
	return nil
}

func UpdateTradeStatus(db *sql.DB, groupID string, entryStatus, stopStatus, ptStatus string) error {
	query := `
        UPDATE trades
        SET
            entry_status = $1,
            stop_status = $2,
            pt_status = $3,
            updated_at = $4
        WHERE group_id = $5`
	_, err := db.Exec(query,
		entryStatus,
		stopStatus,
		ptStatus,
		time.Now(),
		groupID)

	return err
}
