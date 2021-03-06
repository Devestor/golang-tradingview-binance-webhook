package models

import "time"

type EnvConfig struct {
	BinanceAPIKey        string
	BinanceAPISecret     string
	Leverage             int
	TakeProfitPercentage float64
	StopLossPercentage   float64
	LimitMarginSize      float64
	WinOrLossRatio       float64
	Port                 string
	TokenWhitelist       []string
	LineNotifyToken      string
}

type OrderBook struct {
	Side      string
	TimeStamp time.Time
}
