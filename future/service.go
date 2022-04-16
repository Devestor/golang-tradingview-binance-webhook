package future

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"

	"github.com/adshao/go-binance/v2/futures"

	"tradingview-binance-webhook/models"
)

type Service interface {
	Long(command *models.Command) error
	Short(command *models.Command) error

	tradeSetup(command *models.Command)
	openOrder(symbol, quantity string, side futures.SideType, positionSide futures.PositionSideType)
	getDecimalsInfo(symbol string) (int, int)
	calculateQuantity(symbol string, amountUSD int64, quantityPrecision int) float64
	calculateTpSL(symbol string, side futures.PositionSideType, pricePrecision int) (string, string)
	cancelOpenOrders(command models.Command)
}

type service struct {
	config *models.EnvConfig
	client *futures.Client
}

func NewService(
	config *models.EnvConfig,
	client *futures.Client,
) Service {
	return &service{
		config: config,
		client: client,
	}
}

func (s *service) Long(command *models.Command) error {

	// Check Whitelist
	var isFoundTokenWL bool
	for i := range s.config.TokenWhitelist {
		if s.config.TokenWhitelist[i] == command.Symbol {
			isFoundTokenWL = true
			break
		}
	}

	if !isFoundTokenWL {
		return errors.New("Not found in whlitelist token")
	}

	// Setup
	s.tradeSetup(command)

	// GetDecimalsInfo
	pricePrecision, quantityPrecision := s.getDecimalsInfo(command.Symbol)

	quantity := s.calculateQuantity(
		command.Symbol,
		command.AmountUSD,
		quantityPrecision,
	)

	// Open Order
	s.openOrder(command.Symbol, fmt.Sprintf("%.0f", quantity), futures.SideTypeBuy, futures.PositionSideTypeLong)

	// Check is Enable SL or TP
	if !command.IsSL && !command.IsTP {
		return nil
	}

	// Calcualte TP and SL
	stopLoss, takeProfit := s.calculateTpSL(command.Symbol, futures.PositionSideTypeLong, pricePrecision)

	// Enable TakeProfit
	if command.IsTP {
		futureOrder, err := s.client.NewCreateOrderService().
			Symbol(command.Symbol).
			Side(futures.SideTypeSell).
			PositionSide(futures.PositionSideTypeLong).
			Type(futures.OrderTypeTakeProfitMarket).
			StopPrice(takeProfit).
			ClosePosition(true).
			TimeInForce(futures.TimeInForceTypeGTC).
			WorkingType(futures.WorkingTypeMarkPrice).
			PriceProtect(true).
			Do(context.Background())
		if err != nil {
			fmt.Println("Long TP: ", err, ", TP: ", takeProfit)
			return nil
		}
		fmt.Printf("Enable take profit: %s\n", takeProfit)
		fmt.Printf("%+v\n", futureOrder)
	}

	// Enable Stop Loss
	if command.IsSL {
		futureOrder, err := s.client.NewCreateOrderService().
			Symbol(command.Symbol).
			Side(futures.SideTypeSell).
			PositionSide(futures.PositionSideTypeLong).
			Type(futures.OrderTypeStopMarket).
			StopPrice(stopLoss).
			ClosePosition(true).
			TimeInForce(futures.TimeInForceTypeGTC).
			WorkingType(futures.WorkingTypeMarkPrice).
			PriceProtect(true).
			Do(context.Background())
		if err != nil {
			fmt.Println("Long SL: ", err)
			return nil
		}
		fmt.Printf("Enable stop loss: %s\n", stopLoss)
		fmt.Printf("%+v\n", futureOrder)
	}

	return nil
}

func (s *service) Short(command *models.Command) error {

	// Check Whitelist
	var isFoundTokenWL bool
	for i := range s.config.TokenWhitelist {
		if s.config.TokenWhitelist[i] == command.Symbol {
			isFoundTokenWL = true
			break
		}
	}

	if !isFoundTokenWL {
		return errors.New("Not found in whlitelist token")
	}

	// Setup
	s.tradeSetup(command)

	// GetDecimalsInfo
	pricePrecision, quantityPrecision := s.getDecimalsInfo(command.Symbol)

	quantity := s.calculateQuantity(
		command.Symbol,
		command.AmountUSD,
		quantityPrecision,
	)

	// Open Order
	s.openOrder(command.Symbol, fmt.Sprintf("%.0f", quantity), futures.SideTypeSell, futures.PositionSideTypeShort)

	// Check is Enable SL or TP
	if !command.IsSL && !command.IsTP {
		return nil
	}

	// Calcualte TP and SL
	stopLoss, takeProfit := s.calculateTpSL(command.Symbol, futures.PositionSideTypeShort, pricePrecision)

	// Enable TakeProfit
	if command.IsTP {
		futureOrder, err := s.client.NewCreateOrderService().
			Symbol(command.Symbol).
			Side(futures.SideTypeBuy).
			PositionSide(futures.PositionSideTypeShort).
			Type(futures.OrderTypeTakeProfitMarket).
			StopPrice(takeProfit).
			ClosePosition(true).
			TimeInForce(futures.TimeInForceTypeGTC).
			WorkingType(futures.WorkingTypeMarkPrice).
			PriceProtect(true).
			Do(context.Background())
		if err != nil {
			fmt.Println("Short TP: ", err)
			return nil
		}
		fmt.Printf("Enable take profit: %s\n", takeProfit)
		fmt.Printf("%+v\n", futureOrder)
	}

	// Enable Stop Loss
	if command.IsSL {
		futureOrder, err := s.client.NewCreateOrderService().
			Symbol(command.Symbol).
			Side(futures.SideTypeBuy).
			PositionSide(futures.PositionSideTypeShort).
			Type(futures.OrderTypeStopMarket).
			StopPrice(stopLoss).
			ClosePosition(true).
			TimeInForce(futures.TimeInForceTypeGTC).
			WorkingType(futures.WorkingTypeMarkPrice).
			PriceProtect(true).
			Do(context.Background())
		if err != nil {
			fmt.Println("Short SL: ", err)
			return nil
		}
		fmt.Printf("Enable stop loss: %s\n", stopLoss)
		fmt.Printf("%+v\n", futureOrder)
	}
	return nil
}

func (s *service) tradeSetup(command *models.Command) {

	// Change Leverage
	respChangeLeverage, err := s.client.NewChangeLeverageService().
		Symbol(command.Symbol).
		Leverage(s.config.Leverage).
		Do(context.Background())
	if err != nil {
		fmt.Println("Change Leverage: ", err)
		return
	}
	fmt.Printf("Symbol: %s, Leverage: %d, MaxNotionalValue: %s\n", respChangeLeverage.Symbol, respChangeLeverage.Leverage, respChangeLeverage.MaxNotionalValue)

	// Change Margin Type
	err = s.client.NewChangeMarginTypeService().
		Symbol(command.Symbol).
		MarginType(futures.MarginTypeIsolated).
		Do(context.Background())
	if err != nil {
		fmt.Println("Change Margin Type: ", err)
		// return
	}

	// Change Position Mode
	err = s.client.NewChangePositionModeService().DualSide(true).Do(context.Background())
	if err != nil {
		fmt.Println("Change Position Mode: ", err)
		// return
	}
}

func (s *service) openOrder(symbol, quantity string, side futures.SideType, positionSide futures.PositionSideType) {
	// Start Trade
	futureOrder, err := s.client.NewCreateOrderService().
		Symbol(symbol).
		Quantity(quantity).
		Side(side).                 //futures.SideTypeBuy
		PositionSide(positionSide). //futures.PositionSideTypeLong
		Type(futures.OrderTypeMarket).
		Do(context.Background())
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Opened position: %+v\n", futureOrder)
}

func (s *service) getDecimalsInfo(symbol string) (int, int) {
	exchangeInfo, err := s.client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		fmt.Println("GetDecimalsInfo: ", err)
	}

	var pricePrecision, quantityPrecision int
	for i := range exchangeInfo.Symbols {
		if exchangeInfo.Symbols[i].Symbol == symbol {
			pricePrecision = exchangeInfo.Symbols[i].PricePrecision
			quantityPrecision = exchangeInfo.Symbols[i].QuantityPrecision
			break
		}
	}

	fmt.Printf("Symbol: %s, Price Precision: %d, Quantity Precision: %d\n", symbol, pricePrecision, quantityPrecision)
	return pricePrecision, quantityPrecision
}

func (s *service) calculateQuantity(symbol string, amountUSD int64, quantityPrecision int) float64 {

	fff, err := s.client.NewPremiumIndexService().
		Symbol(symbol).
		Do(context.Background())
	if err != nil {
		fmt.Println(err)
		return 0
	}

	markPrice := fff[0].MarkPrice
	var currentPrice float64
	if s, err := strconv.ParseFloat(markPrice, 64); err == nil {
		currentPrice = s
	}

	quantity := toFixed(float64(amountUSD)/currentPrice, quantityPrecision)
	leverageQuantity := quantity * float64(s.config.Leverage)
	fmt.Printf("Default Quantity: %f, Leverage Quantity: %f\n", quantity, leverageQuantity)
	return leverageQuantity
}

func (s *service) calculateTpSL(symbol string, side futures.PositionSideType, pricePrecision int) (string, string) {
	res1, err := s.client.NewGetPositionRiskService().Symbol(symbol).Do(context.Background())
	if err != nil {
		log.Println("CalculateTpSL: ", err)
	}
	var position *futures.PositionRisk
	for _, v := range res1 {
		if v.PositionSide == string(side) {
			fmt.Println()
			position = v
			break
		}
	}
	// fmt.Printf("Entry Price: %s\n", position.EntryPrice)

	price := position.EntryPrice
	fPrice, err := strconv.ParseFloat(price, 64)
	if err != nil {
		fmt.Println("CalculateTpSL2: ", err)
	}

	if side == "LONG" {

		stopLoss := (fPrice * (100 - s.config.StopLossPercentage)) / 100
		takeProfit := (fPrice * (100 + s.config.TakeProfitPercentage)) / 100

		fmt.Printf("Long| Stop Loss: %s, Take Profit: %s\n", fmt.Sprintf("%f", toFixed(stopLoss, pricePrecision)), fmt.Sprintf("%f", toFixed(takeProfit, pricePrecision)))
		return fmt.Sprintf("%f", toFixed(stopLoss, pricePrecision)), fmt.Sprintf("%f", toFixed(takeProfit, pricePrecision))
	} else if side == "SHORT" {

		stopLoss := (fPrice * (100 + s.config.StopLossPercentage)) / 100
		takeProfit := (fPrice * (100 - s.config.TakeProfitPercentage)) / 100

		fmt.Printf("Short| Stop Loss: %s, Take Profit: %s\n", fmt.Sprintf("%f", toFixed(stopLoss, pricePrecision)), fmt.Sprintf("%f", toFixed(takeProfit, pricePrecision)))
		return fmt.Sprintf("%f", toFixed(stopLoss, pricePrecision)), fmt.Sprintf("%f", toFixed(takeProfit, pricePrecision))
	}

	return "0", "0"
}

func (s *service) cancelOpenOrders(command models.Command) {
	err := s.client.NewCancelAllOpenOrdersService().Symbol(command.Symbol).Do(context.Background())
	if err != nil {
		fmt.Println("CancelOpenOrders: ", err)
	}
}

func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func toFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num*output)) / output
}
