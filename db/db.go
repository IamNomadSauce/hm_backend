package db

import (
	"backend/model"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	_ "time"

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
		log.Printf("Error loading .env file %v\n", err)

	}
	host = os.Getenv("PG_HOST")
	portStr := os.Getenv("PG_PORT")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Printf("Invalid port number: %v\n", err)
		return nil, err
	}
	user = os.Getenv("PG_USER")
	password = os.Getenv("PG_PASS")
	dbname = os.Getenv("PG_DBNAME")

	//log.Printf("Host: %s\nPort: %d\nUser: %s\nPW: %s\nDB: %s\n", host, port, user, password, dbname)

	// Connect to the default 'postgres' database to check for the existence of the target database
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)

	DB, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Println("Error opening Postgres", err)
		return nil, err
	}

	log.Println("Successfully connected to database")
	return DB, nil
}

func CreateTables(db *sql.DB) error {
	log.Println("\n------------------------------\n CreateTables \n------------------------------\n")

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
			orderid VARCHAR(255) UNIQUE NOT NULL, 
			productid VARCHAR(255) NOT NULL, 
			tradetype VARCHAR(255) NOT NULL, 
			side VARCHAR(25) NOT NULL, 
			price NUMERIC NOT NULL, 
			size NUMERIC,
			xch_id INTEGER REFERENCES exchanges(id),
			marketcategory varchar(25) NOT NULL, 
			time BIGINT NOT NULL 
		);
		
	`)
	if err != nil {
		log.Println("Failed to create orders table: ", err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS fills (
			entryID VARCHAR(255) UNIQUE NOT NULL, 
			tradeID VARCHAR(255) NOT NULL, 
			orderID VARCHAR(255) NOT NULL, 
			tradeType VARCHAR(25) NOT NULL, 
			price NUMERIC NOT NULL, 
			size NUMERIC NOT NULL,
			side VARCHAR(25) NOT NULL,
			commission NUMERIC NOT NULL,
			productID NUMERIC NOT NULL,
			xch_id INTEGER REFERENCES exchanges(id),
			marketcategory varchar(25) NOT NULL, 
			time BIGINT NOT NULL 
		);
	`)
	if err != nil {
		log.Println("Failed to create fills table: ", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS watchlist (
			id SERIAL PRIMARY KEY,
			xch_id INTEGER REFERENCES exchanges(id),
			product VARCHAR(50) NOT NULL
		);
	`)
	if err != nil {
		log.Println("Failed to create watchlist table: ", err)
	}

	return nil
}

func ListTables(db *sql.DB) error {
	log.Println("\n------------------------------\n ListTables \n------------------------------\n")
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
		log.Println(" -", tableName)
	}

	return nil
}

func Write_Order(orders []model.Order, db *sql.DB) { // Current Orders for all accounts
	log.Println("\n------------------------------\n Write Order \n------------------------------\n")

	_, err := db.Exec("DELECT FROM Orders;")
	if err != nil {
		fmt.Sprintf("Failed to delect existing orders: \n%v", err)
	}

	insertQuery := `
	INSERT INTO orders (orderid, productid, tradetype, side, time, endpoint, marketcategory, price, size)
	VALUES(?,?,?,?,?,?,?,?,?);
	`
	for _, order := range orders {
		_, err := db.Exec(insertQuery, order.OrderID, order.ProductID, order.TradeType, order.Side, order.Timestamp, order.XchID, order.MarketCategory, order.Price, order.Size)
		if err != nil {
			fmt.Sprintf("Error inserting into Order table: \n%v", err)

		}
	}

	log.Println(len(orders), "orders added to db")
}

func Write_Fill(fills []model.Fill, db *sql.DB) {
	log.Println("\n------------------------------\n Write Fills \n------------------------------\n")

	insertQuery := `
	REPLACE INTO fills (entryID, tradeID, orderID, time, tradetype, price, size, side, commission, productid, xch_id, marketcategory);
	`

	for _, fill := range fills {
		_, err := db.Exec(insertQuery, fill.EntryID, fill.TradeID, fill.OrderID, fill.Timestamp, fill.TradeType, fill.Price, fill.Size, fill.Side, fill.Commission, fill.ProductID, fill.XchID, fill.MarketCategory)
		if err != nil {
			fmt.Sprintf("Error inserting fill: \n%v", err)
		}
	}

	log.Println(len(fills), "fills added to db successfully")
}

// ---------------------------------------------------------------

func Write_Candles(candles []model.Candle, product, exchange, tf string, db *sql.DB) error {
	log.Println("\n------------------------------\n Write Candles \n------------------------------\n")
	log.Println(product, tf, exchange, len(candles))

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

	log.Printf("Total candles inserted or updated: %d\n", successCount)
	return nil
}

func Add_Watchlist(product, exchange string) error {

	return nil
}

func Get_Exchange(id int, db *sql.DB) (model.Exchange, error) {
	log.Printf("\n-------------------------------------\n Get Exchange  %v\n-------------------------------------\n", id)

	var exchange model.Exchange

	xch_row, err := db.Query("SELECT id, name FROM exchanges WHERE id = $1", id)
	if err != nil {
		log.Println("Error retrieving exchange from id")
	}
	defer xch_row.Close()

	if err := xch_row.Scan(&exchange.ID, &exchange.Name); err != nil {
		return exchange, fmt.Errorf("Error scanning exchange: %w", err)
	}

	return exchange, nil
}

func Get_Exchanges(db *sql.DB) ([]model.Exchange, error) {
	log.Printf("\n-------------------------------------\n Get All Exchanges \n-------------------------------------\n")

	var exchanges []model.Exchange

	rows, err := db.Query("SELECT * FROM exchanges")
	if err != nil {
		return nil, fmt.Errorf("Error getting exchanges: %v", err)
	}

	for rows.Next() {
		var exchange model.Exchange

		if err := rows.Scan(&exchange.ID, &exchange.Name); err != nil {
			return nil, fmt.Errorf("Error scanning exchange: %w", err)
		}
		exchange.Timeframes, err = Get_Timeframes(exchange.ID, db)
		if err != nil {
			return nil, fmt.Errorf("Error getting timeframes: %w", err)
		}
		exchange.Orders, err = Get_Orders(exchange.ID, db)
		if err != nil {
			return nil, fmt.Errorf("Error getting orders: %w", err)
		}
		exchange.Fills, err = Get_Fills(exchange.ID, db)
		if err != nil {
			return nil, fmt.Errorf("Error getting fills: %w", err)
		}
		exchange.Watchlist, err = Get_Watchlist(exchange.ID, db)
		if err != nil {
			return nil, fmt.Errorf("Error getting watchlist: %w", err)
		}

		switch exchange.Name {
		case "Coinbase":
			exchange.API = &model.CoinbaseAPI{
				APIKey:    os.Getenv("COINBASE_API_KEY"),
				APISecret: os.Getenv("COINBASE_API_SECRET"),
				BaseURL:   "https://api.coinbase.com",
			}
		case "Alpaca":
		}
		// log.Print("\nEXCHANGE\n", exchange)
		// log.Print("\nWATCHLIST\n", exchange.Watchlist)
		exchanges = append(exchanges, exchange)

	}

	return exchanges, nil
}

func Get_Orders(id int, db *sql.DB) ([]model.Order, error) {
	fmt.Sprintf("\n-------------------------------------\n Get Orders  %v\n-------------------------------------\n", id)

	var orders []model.Order
	orderRows, err := db.Query("SELECT orderid, productid, tradetype, side, price, size, xch_id, marketcategory, time FROM orders WHERE xch_id = $1", id)
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

func Get_Fills(id int, db *sql.DB) ([]model.Fill, error) {
	fmt.Sprintf("\n-------------------------------------\n Get Fills  %v\n-------------------------------------\n", id)

	var fills []model.Fill
	fillRows, err := db.Query("SELECT entryid, tradeid, orderid, tradetype, price, size, side, commission, productid, xch_id, marketcategory, time FROM fills WHERE xch_id = $1", id)
	if err != nil {
		fmt.Sprintf("Error retrieving fills for %d \n%v", id, err)
	}

	defer fillRows.Close()

	for fillRows.Next() {
		var fill model.Fill

		if err := fillRows.Scan(&fill.EntryID, &fill.TradeID, &fill.OrderID, &fill.TradeType, &fill.Price, &fill.Size, &fill.Side, &fill.Commission, &fill.ProductID, &fill.XchID, &fill.MarketCategory, &fill.Timestamp); err != nil {
			return fills, fmt.Errorf("Error scanning fill %w", err)
		}
		fills = append(fills, fill)
	}

	return fills, nil

}

func Get_Watchlist(id int, db *sql.DB) ([]model.Product, error) {
	log.Printf("\n-------------------------------------\n Get Watchlist  %v\n-------------------------------------\n", id)

	var watchlist []model.Product

	watchlistRows, err := db.Query("SELECT id, product, xch_id FROM watchlist WHERE xch_id = $1", id)
	if err != nil {
		return watchlist, fmt.Errorf("Error retrieving watchlist: %w", err)
	}

	defer watchlistRows.Close()

	for watchlistRows.Next() {
		var ticker model.Product

		if err := watchlistRows.Scan(&ticker.ID, &ticker.Name, &ticker.XchID); err != nil {
			return watchlist, fmt.Errorf("Error scanning watchlist: %w", err)
		}
		watchlist = append(watchlist, ticker)

	}
	log.Println(watchlist)
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
