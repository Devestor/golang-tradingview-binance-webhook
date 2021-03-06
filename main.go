package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/jasonlvhit/gocron"
	"github.com/joho/godotenv"

	"tradingview-binance-webhook/client"
	"tradingview-binance-webhook/future"
	_lineService "tradingview-binance-webhook/line/service"
	"tradingview-binance-webhook/models"
	"tradingview-binance-webhook/server"
)

var config models.EnvConfig

func init() {

	var err error
	err = godotenv.Load()
	if err != nil {
		log.Fatalf("Error getting env, not comming through %v", err)
	} else {
		log.Println("We are getting the env values")
	}

	config = models.EnvConfig{
		BinanceAPIKey:    os.Getenv("BINANCE_API_KEY"),
		BinanceAPISecret: os.Getenv("BINANCE_API_SECRET"),
		Port:             os.Getenv("PORT"),
		LineNotifyToken:  os.Getenv("LINE_NOTIFY_TOKEN"),
	}

	intLeverage, _ := strconv.ParseInt(os.Getenv("LEVERAGE"), 0, 8)
	config.Leverage = int(intLeverage)

	if f, err := strconv.ParseFloat(os.Getenv("TAKE_PROFIT_PERCENTAGE"), 32); err == nil {
		config.TakeProfitPercentage = f
	}

	if f, err := strconv.ParseFloat(os.Getenv("STOP_LOSS_PERCENTAGE"), 32); err == nil {
		config.StopLossPercentage = f
	}

	if f, err := strconv.ParseFloat(os.Getenv("LIMIT_MARGIN_SIZE"), 32); err == nil {
		config.LimitMarginSize = f
	}

	if f, err := strconv.ParseFloat(os.Getenv("WIN_OR_LOSS_RATIO"), 32); err == nil {
		config.WinOrLossRatio = f
	}

	tokenWhitelist := strings.Split(os.Getenv("TOKEN_WHITELIST"), ",")
	config.TokenWhitelist = tokenWhitelist
}

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU())

	// Http Client
	c := &http.Client{
		Timeout: time.Duration(30 * time.Second),
	}
	cc := client.NewClient(c)

	// NewScheduler
	scheduler := gocron.NewScheduler()
	location, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		log.Println("Unfortunately can't load a location")
	} else {
		scheduler.ChangeLoc(location)
	}

	// Line
	lineService := _lineService.NewLineService(config.LineNotifyToken, cc)

	stateOrderBooks := make(map[string]*models.OrderBook)

	futuresClient := binance.NewFuturesClient(config.BinanceAPIKey, config.BinanceAPISecret) // USDT-M Futures

	// Services
	futureSvc := future.NewService(&config, stateOrderBooks, futuresClient, lineService, scheduler)

	// Server
	srv := server.New(futuresClient, futureSvc)

	errs := make(chan error, 2)
	go func() {
		log.Println("transport", "http", "address", ":"+config.Port, "msg", "listening")
		errs <- http.ListenAndServe(":"+config.Port, srv)
	}()
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT)
		errs <- fmt.Errorf("%s", <-c)
	}()

	log.Println("terminated", <-errs)

}
