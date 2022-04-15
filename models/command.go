package models

import (
	"github.com/adshao/go-binance/v2/futures"
)

type Command struct {
	Symbol    string
	Side      futures.PositionSideType
	AmountUSD int64
	IsTP      bool
	IsSL      bool
}
