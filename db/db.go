package db

import (
    "database/sql"
    "fmt"
    "github.com/lib/pq"
    "os"
    "github.com/joho/godotenv"
    "strconv"
    "time"
)

type Candle struct {
    Time   int64
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
    // fmt.Printf("Host:\n%s\nPort:\n%d\nUser:\n%s\nPW:\n%s\nDB:\n%s\n", host, port, user, password, dbname)
    port, err := strconv.Atoi(portStr)
    if err != nil {
        fmt.Printf("Invalid port number: %v\n", err)
        return nil, err
    }
    user = os.Getenv("PG_USER")
    password = os.Getenv("PG_PASS")
    dbname = os.Getenv("PG_DBNAME")

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

type Order struct {
	OrderID 		string // Exchange specific order identifier
	ProductID		string // xbt_usd_15
	TradeType		string // Long / Short
	Side			string // buy / sell
	Time			int64  // 1724459850
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
	INSERT INTO orders (orderID, productID, tradetype, side, time, exchange, marketcategory, price, size)
	VALUES(?,?,?,?,?,?,?,?,?);
	`
	for _, order := range orders {
		_, err := db.Exec(insertQuery, order.OrderID, order.ProductID, order.TradeType, order.Side, order.Time, order.Exchange, order.MarketCategory, order.Price, order.Size)
		if err != nil {
			fmt.Sprintf("Error inserting into Order table: \n%v", err)

		}
	}

	fmt.Println(len(orders), "orders added to db")
}

type Fill struct {
	EntryID		string
	TradeID		string
	OrderID		string
	Time		int64
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
		_, err := db.Exec(insertQuery, fill.EntryID, fill.TradeID, fill.OrderID, fill.Time, fill.TradeType, fill.Price, fill.Size, fill.Side, fill.Commission, fill.ProductID, fill.Exchange, fill.MarketCategory)
		if err != nil {
			fmt.Sprintf("Error inserting fill: \n%v", err)
		}
	}

	fmt.Println(len(fills), "fills added to db successfully")
}










