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
	ID					int 	`db:"id"`
	XchID 				int 	`db:"xch_id"`
	TF 					string 	`db:"label"`
	Endpoint   			string 	`db:"endpoint"`
	Minutes    			int 	`db:"minutes"`
}

type Exchange struct {
	ID					int 	`db:"id"`
	Name				string 	`db:"name"`
}

type Watchlist struct {
	ID					int		`db:"id"`
	Product				string  `db:"product"`
	Xch_id				int 	`db:"xch_id"`
}

type Order struct {
	Timestamp			int64  `db:"time"` 
	OrderID 			string `db:"orderid"`  // Exchange specific order identifier
	ProductID			string `db:"productid"` // xbt_usd_15
	TradeType			string `db:"tradetype"` // Long / Short
	Side				string `db:"side"` // buy / sell
	XchID				int    `db:"xch_id"`
	MarketCategory		string `db:"marketcategory"` // (crypto / equities)_(spot / futures)
	Price				string `db:"price"` // instrument_currency
	Size				string `db:"size"` // How many of instrument
}

type Fill struct {
	Timestamp		int		`db:"time"` 
	EntryID			string	`db:"entryid"` 
	TradeID			string	`db:"tradeid"` 
	OrderID			string	`db:"orderid"` 
	TradeType		string	`db:"tradetype"` 
	Price			string	`db:"price"` 
	Size			string	`db:"size"` 
	Side			string	`db:"side"` 
	Commission		string	`db:"commission"` 
	ProductID		string	`db:"productid"` 
	XchID			int		`db:"xch_id"` 
	MarketCategory	string  `db:"marketcategory"`
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
		CREATE TABLE IF NOT EXISTS exchanges (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL
		);
	`)
	if err != nil { 
		fmt.Println("Failed to create exchanges table: ", err)
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
		fmt.Println("Failed to create timeframes table: ", err)
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
			xch_id INTEGER REFERENCES exchanges(id),
			marketcategory varchar(25) NOT NULL, 
			time BIGINT NOT NULL 
		);
	`)
	if err != nil { 
		fmt.Println("Failed to create fills table: ", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS watchlist (
			id SERIAL PRIMARY KEY,
			xch_id INTEGER REFERENCES exchanges(id),
			product VARCHAR(50) NOT NULL
		);
	`)
	if err != nil { 
		fmt.Println("Failed to create watchlist table: ", err)
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
	INSERT INTO orders (orderid, productid, tradetype, side, time, endpoint, marketcategory, price, size)
	VALUES(?,?,?,?,?,?,?,?,?);
	`
	for _, order := range orders {
		_, err := db.Exec(insertQuery, order.OrderID, order.ProductID, order.TradeType, order.Side, order.Timestamp, order.XchID, order.MarketCategory, order.Price, order.Size)
		if err != nil {
			fmt.Sprintf("Error inserting into Order table: \n%v", err)

		}
	}

	fmt.Println(len(orders), "orders added to db")
}

func Write_Fill(fills []Fill) {
	fmt.Println("\n------------------------------\n Write Fills \n------------------------------\n")

	db, _ := DBConnect()
	defer db.Close()
	
	insertQuery := `
	REPLACE INTO fills (entryID, tradeID, orderID, time, tradetype, price, size, side, commission, productid, xch_id, marketcategory);
	`

	for _, fill := range fills {
		_, err := db.Exec(insertQuery, fill.EntryID, fill.TradeID, fill.OrderID, fill.Timestamp, fill.TradeType, fill.Price, fill.Size, fill.Side, fill.Commission, fill.ProductID, fill.XchID, fill.MarketCategory)
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

type Account struct {
	Exchange 	Exchange
	Timeframes	[]Timeframe
	Orders 		[]Order
	Fills		[]Fill
	Watchlist	[]Watchlist
}

func Get_Account(id int, db *sql.DB) (Account, error) { 
	fmt.Sprintf("\n-------------------------------------\n Get Account %v\n-------------------------------------\n", id)

}

func Get_Exchange(id int) ( []Exchange, error) {
	fmt.Sprintf("\n-------------------------------------\n Get Exchange  %v\n-------------------------------------\n", id)
}

func Get_Orders(id int) ([]Order, error) {
	fmt.Sprintf("\n-------------------------------------\n Get Orders  %v\n-------------------------------------\n", id)
}

func Get_Fills(id int) ([]Order, error) {
	fmt.Sprintf("\n-------------------------------------\n Get Fills  %v\n-------------------------------------\n", id)
}

func Get_Watchlist(id int) ([]Watchlist, error) {
	fmt.Sprintf("\n-------------------------------------\n Get Watchlist  %v\n-------------------------------------\n", id)
}

func Get_Timeframes(id int) ([]Timeframe, error) {
	fmt.Sprintf("\n-------------------------------------\n Get Timeframes  %v\n-------------------------------------\n", id)
}


