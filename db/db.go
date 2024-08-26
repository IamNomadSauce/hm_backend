package db

import (
    "database/sql"
    "fmt"
    _"github.com/lib/pq"
    "os"
    "github.com/joho/godotenv"
    "strconv"
    _"time"
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

var host string
var port int
var user string
var password string
var dbname string

func DBConnect() (*sql.DB, error) {
	
	fmt.Println("\n------------------------------\n DBConnect \n------------------------------\n")
	err := godotenv.Load()
	if err != nil {
	fmt.Printf("Error loading .env file %v\n", err)

	}
    host = os.Getenv("PG_HOST")
    portStr := os.Getenv("PG_PORT")
    port, err := strconv.Atoi(portStr)
    if err != nil {
        fmt.Printf("Invalid port number: %v\n", err)
        return nil, err
    }
    user = os.Getenv("PG_USER")
    password = os.Getenv("PG_PASS")
    dbname = os.Getenv("PG_DBNAME")

    fmt.Printf("Host: %s\nPort: %d\nUser: %s\nPW: %s\nDB: %s\n", host, port, user, password, dbname)

    // Connect to the default 'postgres' database to check for the existence of the target database
    psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)

    db, err := sql.Open("postgres", psqlInfo)
    if err != nil {
        fmt.Println("Error opening Postgres", err)
        return nil, err
    }
    //defer db.Close()

    return db, nil

}

func CreateTables(db *sql.DB) error {
	fmt.Println("\n------------------------------\n CreateTables \n------------------------------\n")

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			orderid VARCHAR(255) UNIQUE NOT NULL, 
			productid VARCHAR(255) NOT NULL, 
			tradetype VARCHAR(255) NOT NULL, 
			side VARCHAR(25) NOT NULL, 
			price NUMERIC NOT NULL, 
			size NUMERIC,
			exchange NUMERIC NOT NULL, 
			marketcategory varchar(25) NOT NULL, 
			time BIGINT NOT NULL 
		);
		
	`)
	if err != nil { 
		fmt.Println("Failed to create orders table: ", err)
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
			exchange VARCHAR(25) NOT NULL, 
			marketcategory varchar(25) NOT NULL, 
			time BIGINT NOT NULL 
		);
	`)
	if err != nil { 
		fmt.Println("Failed to create fills table: ", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS exchanges (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil { 
		fmt.Println("Failed to create exchanges table: ", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS timeframe_sets (
			id SERIAL PRIMARY KEY,
			exchange_id INTEGER REFERENCES exchanges(id),
			name VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil { 
		fmt.Println("Failed to create timeframe_sets table: ", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS timeframes (
			id SERIAL PRIMARY KEY,
			set_id INTEGER REFERENCES exchanges(id),
			label VARCHAR(10) NOT NULL,
			tf INTEGER NOT NULL,
			xch VARCHAR(50) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil { 
		fmt.Println("Failed to create timeframes table: ", err)
	}

	return nil
}

func ListTables(db *sql.DB) error {
	fmt.Println("\n------------------------------\n ListTables \n------------------------------\n")
	rows, err := db.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'")
	if err != nil {
		fmt.Println("Error listing tables", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil{
			fmt.Println("Error scanning table name", err)
			return err
		}
		fmt.Println(" -", tableName)
	}

	return nil
}


type Order struct {
	Timestamp			int64  // 1724459850
	OrderID 		string // Exchange specific order identifier
	ProductID		string // xbt_usd_15
	TradeType		string // Long / Short
	Side			string // buy / sell
	Exchange		string // coinbase / alpaca
	MarketCategory		string // (crypto / equities)_(spot / futures)
	Price			string // instrument_currency
	Size			string // How many of instrument
}

func Write_Order(orders []Order) { // Current Orders for all accounts
	fmt.Println("\n------------------------------\n Write Order \n------------------------------\n")
	
	db, err := DBConnect()
	if err != nil {
		fmt.Sprintf("Error connecting to db %v", err)
	}
	
	defer db.Close()

	_, err = db.Exec("DELECT FROM Orders;")
	if err != nil {
		fmt.Sprintf("Failed to delect existing orders: \n%v", err)
	}

	insertQuery := `
	INSERT INTO orders (orderid, productid, tradetype, side, time, exchange, marketcategory, price, size)
	VALUES(?,?,?,?,?,?,?,?,?);
	`
	for _, order := range orders {
		_, err := db.Exec(insertQuery, order.OrderID, order.ProductID, order.TradeType, order.Side, order.Timestamp, order.Exchange, order.MarketCategory, order.Price, order.Size)
		if err != nil {
			fmt.Sprintf("Error inserting into Order table: \n%v", err)

		}
	}

	fmt.Println(len(orders), "orders added to db")
}

type Fill struct {
	Timestamp	int64
	EntryID		string
	TradeID		string
	OrderID		string
	TradeType	string
	Price		string	
	Size		string
	Side		string
	Commission	string
	ProductID	string
	Exchange	string
	MarketCategory	string
}

func Write_Fill(fills []Fill) {
	fmt.Println("\n------------------------------\n Write Fills \n------------------------------\n")

	db, _ := DBConnect()
	defer db.Close()
	
	insertQuery := `
	REPLACE INTO fills (entryID, tradeID, orderID, time, tradetype, price, size, side, commission, productid, exchange, marketcategory);
	`

	for _, fill := range fills {
		_, err := db.Exec(insertQuery, fill.EntryID, fill.TradeID, fill.OrderID, fill.Timestamp, fill.TradeType, fill.Price, fill.Size, fill.Side, fill.Commission, fill.ProductID, fill.Exchange, fill.MarketCategory)
		if err != nil {
			fmt.Sprintf("Error inserting fill: \n%v", err)
		}
	}

	fmt.Println(len(fills), "fills added to db successfully")
}

// ---------------------------------------------------------------

func Write_Candles(candles []Candle, product, exchange string) error {
	fmt.Println("\n------------------------------\n Write Fills \n------------------------------\n")

	db, _ := DBConnect()
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("Failed to begin transaction:  %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(fmt.Sprintf("INSERT INTO %s_%s_%s (timestamp, open, high, low, close, volume) VALUS ($1, $2, $3, $4, $5, $6) ON CONFLICT (timestamp) DO UPDATE SET open = $2, high = $3, low = $4, close = $5, volume = $6", product, exchange))
	if err != nil {
		return fmt.Errorf("Failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, candle := range candles {
		_, err := stmt.Exec(candle.Timestamp, candle.Open, candle.High, candle.Low, candle.Close, candle.Volume)

		if err != nil {
			return fmt.Errorf("Failed to insert candles: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("Failed to commit transaction: %w", err)
	}

	return nil

}

func Add_Watchlist(product, exchange string) error {


	return nil
}







