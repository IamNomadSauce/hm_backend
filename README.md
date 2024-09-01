Back end system for retrieving and storing market candlestick data

hm_backend
-- DB
  |-- db.go
-- API
  |-- coinbase.go
  |-- alpaca.go
-- main.go

TODO:
- Backend
[x] DB Create Tables if not exist
[x] Write Fills
[x] Write Current Orders
[x] Write Candles
[x] Create and populate Exchange data
[ ] Coinbase Fetching account and candles
[ ] Coinbase Add account and candles to db
[ ] Coinbase fetch candles and account data from db 
[ ] main loop - 
    [ ] fetch all candles with all timeframes from all assets and their exchange according to watchlist
    [ ] Portfolio
[ ] api/api.go use for exchange specific structs for available timeframes.
[ ] SQL Events websocket: trigger api call to update backend 
    [ ] portfolio/positions change
    [ ] orders triggered
- Server (serving data to client)
[ ] http API
[ ] websocket stream

------------------

Structs:
Account:
-- portfolio history
-- portfolio / exchange
Exchange:
-- Orders
-- Fills
-- Timeframes
-- Watchlist
