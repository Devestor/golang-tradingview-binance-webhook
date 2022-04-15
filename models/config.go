package models

type EnvConfig struct {
	BinanceAPIKey        string
	BinanceAPISecret     string
	Leverage             int
	TakeProfitPercentage float64
	StopLossPercentage   float64
	Port                 string
	TokenWhitelist       []string
}
