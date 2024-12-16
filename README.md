Back end system for retrieving and storing market candlestick data

Use requires a postgres database

hm_backend
-- DB
  |-- db.go
-- API
  |-- coinbase.go
  |-- alpaca.go
-- main.go

mainloop 
    |> api.fetch_and_store_candles 
        |> exchange.API.FetchCandles 
        |> db.Write_Candles
    |> api.Fetch_and_store_fills
    |> api.FetchOrders
    |> api.Fetch_and_store_portfolio
        |> 
Server

TODO:
--------------------------------------
- [x] Portfolio coinbase retrieval
- [x] Portfolio storage
- [x] Portfolio retrieval db
- [x] Storing fills
- [x] Storing orders
- [ ] Alpaca API
--------------------------------------
- [x] Add Table for available products
- [x] Extend Product struct with more fields
- [x] Fetch available products from db and send to client.
- [x] Display available products in client dropdown
- [x] Create "add" button to add product to watchlist
- [x] If adding a new asset, make sure to have retrieve 30 days of candles.
-------------------------------------------------------------------------
- Backend
- [ ] TradeBlock
    - [ ] Client  
        - [x] Trade lines on chart
        - [x] Send trade-block to backend
        - [x] Receive ok from back-end -> delete lines
        - [ ] Delete trade-block
        - [x] Display trade-block and trades status
    - [ ] Backend
        - [x] Receive trade-block and parse into TradeBlock
        - [x] Split trade-block into a block of trades with base_size, based on len(profit_targets)
        - [x] Write trades to db
        - [ ] Trade loop for each trade in trade-block not completed
            - [x] Place entry order to exchange, assign group_id to order when created
            - [x] When entry filled -> place bracket order
            - [ ] Update trade status when entry, stop, or profit target are filled.
                - [x] Entry
                - [ ] Stoploss
                - [ ] ProfitTarget
        - [x] Add Trade-blocks to Exchange struct


    - [x] DB Create Tables if not exist
    - [x] Write Fills
    - [x] Write Current Orders
    - [x] Write Candles
    - [x] Create and populate Exchange data
    - [x] API
        - [x] Retrieve and send candles to client
    - [ ] Candle Gap Integrity Check
        - [ ] Pass Timeframe data into loops
        - [x] Recursive Coinbase candle retrieval

- [ ] Create and account Exchange data
    - [x] Portfolio balances of each coin.
        - [ ] Write portfolio balance to db
    - [x] fills 
    - [x] Orders 
    - [x] Product price

    - [x] Coinbase Fetching candles (including loop for entire candle history)
        - [x] Get Candles (single run)
        - [x] Recursive all candle history loop
    - [x] Coinbase Write candles to db
        - [x] Create tables based on exchange tfs and watchlist.
        - [x] Write candles to db
    
- [ ] main loop - 
    - [x] fetch all candles with all timeframes from all assets and their exchange according to watchlist
    - [x] Fetch and update Portfolio

    - [ ] api/api.go use for exchange specific structs for available timeframes.
    - [ ] SQL Events websocket: trigger api call to update backend 
    - [ ] portfolio/positions change
    - [ ] orders triggered

- Server (serving data to client)
    - [x] http API
    - [ ] websocket stream
    
- Client:
    - [x] Templates for chart page
        - [x] Navbar
        - [x] Exchanges bar
        - [x] Chart/graph
    - [ ] Websocket (frontend and backend)

- Chart:
    - [ ] Drawing tools
        - [x] Lines
            [ ] Change button color based on active status
        - [x] Boxes
        - [ ] Trendlines
    - [x] Exchange, tf, and asset bar
    - [x] Render candle data
    - [x] Pan & zoom
    - [ ] Websocket rendering

- Exchanges
    - [ ] Coinbase
        - [x] Candle Retrieval and storage from exchange
        - [x] Client candle retrieval
        - [x] Watchlist
        - [x] Account & portfolio balance tracking

    - [ ] Alpaca
        - [ ] Candle Retrieval and storage from exchange
        - [ ] Client candle retrieval
        - [ ] Watchlist
        - [ ] Account & portfolio balance tracking



















------------------

