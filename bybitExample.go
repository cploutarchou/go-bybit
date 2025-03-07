package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/cploutarchou/crypto-sdk-suite/bybit/account"
	"github.com/cploutarchou/crypto-sdk-suite/bybit/client"
	"github.com/cploutarchou/crypto-sdk-suite/bybit/ws"
	wsClient "github.com/cploutarchou/crypto-sdk-suite/bybit/ws/client"
	kline2 "github.com/cploutarchou/crypto-sdk-suite/bybit/ws/public/kline"
	ticker2 "github.com/cploutarchou/crypto-sdk-suite/bybit/ws/public/ticker"
)

var bybitCli *client.Client
var acc account.Account
var websocket ws.WebSocket
var key string
var secret string

func init() {
	key = os.Getenv("BYBIT_FUTURES_TESTNET_API_KEY")
	secret = os.Getenv("BYBIT_FUTURES_TESTNET_API_SECRET")
	bybitCli = client.NewClient(key, secret, true)
	acc = account.New(bybitCli)
}

func getWalletBalance() (any, error) {
	wallet := acc.Wallet()
	fmt.Println("getWalletBalance")
	return wallet.GetContractWalletBalance("BTC")
}

func upgradeToUnified() (any, error) {
	fmt.Println("upgradeToUnified")
	upgrade := acc.UpgradeToUnified()
	return upgrade.Upgrade()
}

func getBorrowHistory() (any, error) {
	fmt.Println("getBorrowHistory")
	borrow := acc.Borrow()
	return borrow.GetHistory("BTC", 0, 0, 0, "")
}

func getCollateralCoin() (any, error) {
	fmt.Println("getCollateralCoin")
	collateral := acc.Collateral()
	return collateral.GetInfo("BTCUSDT")
}

func setCollateralCoin() (any, error) {
	fmt.Println("setCollateralCoin")
	collateral := acc.Collateral()
	return collateral.Set("BTC", "ON")
}

func getCoinGreeks() (any, error) {
	fmt.Println("getCoinGreeks")
	coinGreeks := acc.CoinGreek()
	return coinGreeks.Get("BTC")
}

func getFeeRates() (any, error) {
	fmt.Println("getFeeRates")
	feeRates := acc.FeeRates()
	return feeRates.GetFeeRate("taker", "BTCUSDT", "USDT")
}

func getInfo() (any, error) {
	fmt.Println("getInfo")
	info := acc.Info()
	return info.Get()
}

func getTransactionLog() (any, error) {
	params := map[string]string{
		"accountType": "UNIFIED",
		"category":    "linear",
		"currency":    "USDT",
	}
	fmt.Println("getTransactionLog")
	transactionLog := acc.TransactionLog()
	return transactionLog.Get(params)
}

func setMargin() (any, error) {
	margin := acc.Margin()
	fmt.Println("setMargin")
	return margin.SetMarginMode("ISOLATED")
}

func setMMP() (any, error) {
	margin := acc.Margin()
	params := &account.MMPParams{
		BaseCoin:     "BTC",
		Window:       200,
		FrozenPeriod: 10,
		QtyLimit:     100,
		DeltaLimit:   100,
	}
	fmt.Println("setMMP")
	return margin.SetMMP(params)
}

func resetMMP() (any, error) {
	margin := acc.Margin()
	fmt.Println("resetMMP")
	return margin.ResetMMP("BTC")
}

func getMMPState() (any, error) {
	margin := acc.Margin()
	fmt.Println("getMMPState")
	return margin.GetMMPState("BTC")
}

func wsConnectTicker() {
	b := make(chan float64, 1)
	fmt.Println("wsConnectTicker")
	publicClient, err := wsClient.NewPublicClient(true, "linear")
	if err != nil {
		log.Printf("ERROR: Failed to create WebSocket client: %v", err)
		return
	}

	websocket = ws.New(publicClient, nil, true)
	publicWS, err := websocket.Public()
	if err != nil {
		log.Printf("ERROR: Failed to access public WebSocket endpoint: %v", err)
		return
	}

	ticker := publicWS.Ticker("linear")

	err = ticker.Subscribe("BTCUSDT", func(data ticker2.Data) {
		if data.LastPrice != "" {
			lastPrice, parseErr := strconv.ParseFloat(data.LastPrice, 64)
			if parseErr != nil {
				log.Printf("ERROR: Parsing last price: %v", parseErr)
				return
			}
			select {
			case b <- lastPrice:
			default:
				log.Println("WARNING: Channel is blocked or full, skipping update.")
			}
		}
	})
	if err != nil {
		log.Printf("ERROR: Failed to subscribe to ticker updates: %v", err)
		return
	}

	go ticker.Listen()

	log.Println("INFO: Successfully subscribed to live price updates for BTCUSDT")

	for price := range b {
		log.Printf("Received price update: %f", price)
	}
}

func wsConnectKline() {
	fmt.Println("wsConnectKline")

	client_, err := wsClient.NewPublicClient(true, "linear")
	if err != nil {
		log.Printf("ERROR: Failed to create WebSocket client: %v", err)
		return
	}

	klineService, err := kline2.New(client_)
	if err != nil {
		log.Printf("ERROR: Failed to initialize kline service: %v", err)
		return
	}

	err = klineService.Subscribe([]string{"BTCUSDT", "SOLUSDT"}, "1", func(data kline2.Data) {
		log.Printf("Received kline update: %+v\n", data)
	})
	if err != nil {
		log.Printf("ERROR: Failed to subscribe to kline updates: %v", err)
		return
	}

	log.Println("INFO: Successfully subscribed to kline updates for BTCUSDT and SOLUSDT")

	for kline := range klineService.GetMessagesChan() {
		log.Printf("Received kline update: %+v\n", string(kline))
	}
}

func runAccountExamples() {
	handleErrorWithPrint(getWalletBalance())
	handleErrorWithPrint(upgradeToUnified())
	handleErrorWithPrint(getBorrowHistory())
	handleErrorWithPrint(getCollateralCoin())
	handleErrorWithPrint(setCollateralCoin())
	handleErrorWithPrint(getCoinGreeks())
	handleErrorWithPrint(getFeeRates())
	handleErrorWithPrint(getInfo())
	handleErrorWithPrint(getTransactionLog())
	handleErrorWithPrint(setMargin())
	handleErrorWithPrint(setMMP())
	handleErrorWithPrint(resetMMP())
	handleErrorWithPrint(getMMPState())
	wsConnectTicker()
	wsConnectKline()
}

func bybitExamples() {
	runAccountExamples()
}

func main() {
	bybitExamples()
}
