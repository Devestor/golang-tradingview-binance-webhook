package future

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/jasonlvhit/gocron"

	"tradingview-binance-webhook/line"
	"tradingview-binance-webhook/models"
	"tradingview-binance-webhook/utils"
)

type Service interface {
	Long(command *models.Command) error
	Short(command *models.Command) error
	GetPositionRisk(command *models.Command) (*futures.PositionRisk, error)
	CheckPositionRatio(command *models.Command, positionRisk *futures.PositionRisk) (bool, error)
	calculateRealizedPnl() (*models.CalculateRealizedPnl, error)

	startScheduler()
	listenUserData() error

	tradeSetup(command *models.Command)
	openOrder(symbol, quantity string, side futures.SideType, positionSide futures.PositionSideType)
	getDecimalsInfo(symbol string) (int, int)
	calculateQuantity(symbol string, amountUSD int64, quantityPrecision int) float64
	calculateTpSL(symbol string, side futures.PositionSideType, pricePrecision int) (string, string, error)
	cancelOpenOrders(command models.Command)
}

type service struct {
	stateOrderBooks map[string]*models.OrderBook
	config          *models.EnvConfig
	client          *futures.Client
	lineService     line.Service
	scheduler       *gocron.Scheduler
	listenKey       string
}

func NewService(
	config *models.EnvConfig,
	stateOrderBooks map[string]*models.OrderBook,
	client *futures.Client,
	lineService line.Service,
	scheduler *gocron.Scheduler,
) Service {

	s := &service{
		config:          config,
		stateOrderBooks: stateOrderBooks,
		client:          client,
		lineService:     lineService,
		scheduler:       scheduler,
	}

	// Scheduler
	go s.startScheduler()

	// Web Socket
	go s.listenUserData()

	return s
}

func (s *service) Long(command *models.Command) error {

	// Check Whitelist
	var isFoundTokenWL bool
	if command.IsCheckWL {
		for i := range s.config.TokenWhitelist {
			if s.config.TokenWhitelist[i] == command.Symbol {
				isFoundTokenWL = true
				break
			}
		}

	}

	// Check Token Whitelist
	if !isFoundTokenWL && command.IsCheckWL {
		return errors.New("Not found in whlitelist token")
	}

	// Delay open order
	if s.isDelayOpenOrder(command) {
		log.Println("Delay open order")
		return errors.New("Delay open order")
	}

	// GetPositionRisk
	positionRisk, err := s.GetPositionRisk(command)
	if err != nil {
		return err
	}

	var isolatedWallet float64
	if f, err := strconv.ParseFloat(positionRisk.IsolatedWallet, 64); err == nil {
		isolatedWallet = f
	}

	// If Large position and loss ratio more than config
	if isolatedWallet > s.config.LimitMarginSize {
		if canOpenOrder, err := s.CheckPositionRatio(command, positionRisk); !canOpenOrder {
			log.Println(err)
			return err
		}
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
	stopLoss, takeProfit, err := s.calculateTpSL(command.Symbol, futures.PositionSideTypeLong, pricePrecision)
	if err != nil {
		return err
	}

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
	if command.IsCheckWL {
		for i := range s.config.TokenWhitelist {
			if s.config.TokenWhitelist[i] == command.Symbol {
				isFoundTokenWL = true
				break
			}
		}
	}

	// Check Token Whitelist
	if !isFoundTokenWL && command.IsCheckWL {
		return errors.New("Not found in whlitelist token")
	}

	// Delay open order
	if s.isDelayOpenOrder(command) {
		log.Println("Delay open order")
		return errors.New("Delay open order")
	}

	// GetPositionRisk
	positionRisk, err := s.GetPositionRisk(command)
	if err != nil {
		return err
	}

	var isolatedWallet float64
	if f, err := strconv.ParseFloat(positionRisk.IsolatedWallet, 64); err == nil {
		isolatedWallet = f
	}

	// If Large position and loss ratio more than config
	if isolatedWallet > s.config.LimitMarginSize {
		if canOpenOrder, err := s.CheckPositionRatio(command, positionRisk); !canOpenOrder {
			return err
		}
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
	stopLoss, takeProfit, err := s.calculateTpSL(command.Symbol, futures.PositionSideTypeShort, pricePrecision)
	if err != nil {
		return err
	}

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

	// const { symbol, onlyOneOrder } = command

	// const positionOrders = await this.getPositionOrders(command)

	// if (onlyOneOrder && positionOrders.length > 0) {
	//   return false
	// }

	// await this.cancelOpenOrders(command, positionOrders)

	openOrders, err := s.client.NewListOpenOrdersService().Symbol(command.Symbol).Do(context.Background())
	if err != nil {
		fmt.Println(err)
		return
	}

	if command.OnlyOneOrder && len(openOrders) > 0 {
		fmt.Println("Skipping open order")
		return
	} else if len(openOrders) > 0 {
		for _, o := range openOrders {
			if command.Side == o.PositionSide {
				_, err := s.client.NewCancelOrderService().Symbol(o.Symbol).OrderID(o.OrderID).Do(context.Background())
				if err != nil {
					fmt.Println(err)
					return
				}
				fmt.Println("Cancal Order Symbol: ", o.Symbol, ", Order ID: ", o.OrderID)
			}
		}
	}

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

	quantity := utils.ToFixed(float64(amountUSD)/currentPrice, quantityPrecision)
	leverageQuantity := quantity * float64(s.config.Leverage)
	// fmt.Printf("Default Quantity: %f, Leverage Quantity: %f\n", quantity, leverageQuantity)
	return leverageQuantity
}

func (s *service) calculateTpSL(symbol string, side futures.PositionSideType, pricePrecision int) (string, string, error) {
	res1, err := s.client.NewGetPositionRiskService().Symbol(symbol).Do(context.Background())
	if err != nil {
		log.Println("CalculateTpSL: ", err)
		return "", "", err
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
		return "", "", err
	}

	if side == "LONG" {

		stopLoss := (fPrice * (100 - s.config.StopLossPercentage)) / 100
		takeProfit := (fPrice * (100 + s.config.TakeProfitPercentage)) / 100

		// fmt.Printf("Long| Stop Loss: %s, Take Profit: %s\n", fmt.Sprintf("%f", utils.ToFixed(stopLoss, pricePrecision)), fmt.Sprintf("%f", utils.ToFixed(takeProfit, pricePrecision)))
		return fmt.Sprintf("%f", utils.ToFixed(stopLoss, pricePrecision)), fmt.Sprintf("%f", utils.ToFixed(takeProfit, pricePrecision)), nil
	} else if side == "SHORT" {

		stopLoss := (fPrice * (100 + s.config.StopLossPercentage)) / 100
		takeProfit := (fPrice * (100 - s.config.TakeProfitPercentage)) / 100

		// fmt.Printf("Short| Stop Loss: %s, Take Profit: %s\n", fmt.Sprintf("%f", utils.ToFixed(stopLoss, pricePrecision)), fmt.Sprintf("%f", utils.ToFixed(takeProfit, pricePrecision)))
		return fmt.Sprintf("%f", utils.ToFixed(stopLoss, pricePrecision)), fmt.Sprintf("%f", utils.ToFixed(takeProfit, pricePrecision)), nil
	}

	return "0", "0", nil
}

func (s *service) cancelOpenOrders(command models.Command) {
	err := s.client.NewCancelAllOpenOrdersService().Symbol(command.Symbol).Do(context.Background())
	if err != nil {
		fmt.Println("CancelOpenOrders: ", err)
	}
}

// true = delay, false = not delay
func (s *service) isDelayOpenOrder(command *models.Command) bool {
	var diff time.Duration
	var isFirst bool
	sk := command.Symbol + string(command.Side)
	if s.stateOrderBooks[sk] == nil { // First
		isFirst = true
		s.stateOrderBooks[sk] = &models.OrderBook{
			Side:      string(command.Side),
			TimeStamp: time.Now(),
		}
		return false
	}

	// compare
	diff = time.Since(s.stateOrderBooks[sk].TimeStamp)
	fmt.Println("Key: ", sk)
	fmt.Println("Diff time: ", diff)
	fmt.Println("Diff time seconds: ", diff.Seconds())
	if isFirst || diff.Seconds() >= 330 { // 5.5min
		// Open Order
		fmt.Println("Open Order: Diff is ", diff.Seconds())

		s.stateOrderBooks[sk] = &models.OrderBook{
			Side:      string(command.Side),
			TimeStamp: time.Now(),
		}

		return false
	}

	return true
}

func (s *service) GetPositionRisk(command *models.Command) (*futures.PositionRisk, error) {

	orders, err := s.client.NewGetPositionRiskService().Symbol(command.Symbol).
		Do(context.Background())
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	var result *futures.PositionRisk
	for _, o := range orders {
		if o.PositionSide == string(command.Side) {
			result = o
			break
		}
	}

	return result, nil
}

// True= Can open order, False= Skip open order
func (s *service) CheckPositionRatio(command *models.Command, positionRisk *futures.PositionRisk) (bool, error) {

	var entryPrice, markPrice float64
	if f, err := strconv.ParseFloat(positionRisk.EntryPrice, 64); err == nil {
		entryPrice = f
	}
	if f, err := strconv.ParseFloat(positionRisk.MarkPrice, 64); err == nil {
		markPrice = f
	}

	// Don't have Open order
	if entryPrice == 0 {
		return true, nil
	}

	var isOverRatio bool
	var roe float64

	if command.Side == futures.PositionSideTypeShort { // Short
		roe = (entryPrice - markPrice) / entryPrice * 100
	} else if command.Side == futures.PositionSideTypeLong { // Long
		roe = (markPrice - entryPrice) / markPrice * 100
	}

	if roe < s.config.WinOrLossRatio {
		isOverRatio = true
	}

	// isOverRatio == false
	if !isOverRatio {
		msg := fmt.Sprintf("Skipping Order: Side: %s, Entry Price: %f, Mark Price: %f, ROE: %.2f", command.Side, entryPrice, markPrice, utils.ToFixed(roe, 2))
		return isOverRatio, errors.New(msg)
	}

	return isOverRatio, nil
}

func (s *service) listenUserData() error {
	log.Println("streaming user data...")

	listenKey, err := s.client.NewStartUserStreamService().Do(context.Background())
	if err != nil {
		log.Println(err)
		return err
	}

	s.listenKey = listenKey
	log.Println("listenKey: ", listenKey)

	wsHandler := func(event *futures.WsUserDataEvent) {
		if event.Event == futures.UserDataEventTypeOrderTradeUpdate {
			// TP
			if event.OrderTradeUpdate.ExecutionType == futures.OrderExecutionTypeTrade && event.OrderTradeUpdate.OriginalType == futures.OrderTypeTakeProfitMarket {

				msg := fmt.Sprintf(`%s [%s] 🔴 ปิด position 
กำไร $%s
ค่าคอมมิสชั่น: $%s
			`,
					event.OrderTradeUpdate.Symbol,
					event.OrderTradeUpdate.PositionSide,
					event.OrderTradeUpdate.RealizedPnL,
					event.OrderTradeUpdate.Commission,
				)

				s.lineService.Notify(msg)

				realizedPnl, err := s.calculateRealizedPnl()
				if err != nil {
					log.Println("RealizedPnl: ", err)
				}

				msg2 := fmt.Sprintf(`🔰🚸🎏🧬🧪 Net Profit
กำไร: %.2f
ขาดทุน: %.2f
Commission: %.2f
กำไรรวมวันนี้ %.2f`,
					realizedPnl.Profit,
					realizedPnl.Loss,
					realizedPnl.Commission,
					realizedPnl.NetProfit,
				)

				s.lineService.Notify(msg2)

				log.Printf("### TP 1 ###: %+v\n\n", event)
			} else {
				// Open Order
				if event.OrderTradeUpdate.ExecutionType == futures.OrderExecutionTypeTrade {
					var averagePrice, originalQty float64
					if f, err := strconv.ParseFloat(event.OrderTradeUpdate.AveragePrice, 32); err == nil {
						averagePrice = f
					}
					if f, err := strconv.ParseFloat(event.OrderTradeUpdate.OriginalQty, 32); err == nil {
						originalQty = f
					}

					msg := fmt.Sprintf(`%s [%s] ✅ เปิด position
ราคา: $%s
จำนวน: %s
จำนวนUSD: $%.2f
ค่าคอมมิสชั่น: $%s
					`,
						event.OrderTradeUpdate.Symbol,
						event.OrderTradeUpdate.PositionSide,
						event.OrderTradeUpdate.AveragePrice,
						event.OrderTradeUpdate.OriginalQty,
						averagePrice*originalQty,
						event.OrderTradeUpdate.Commission,
					)

					s.lineService.Notify(msg)
					log.Printf("### OPEN 2 ###: %+v\n\n", event)

				} else {
					s.lineService.Notify(fmt.Sprintf("`## Event: %s, Time: %d, Commission: %s", event.Event, event.Time, event.OrderTradeUpdate.Commission))
					log.Printf("### 3 ###: %+v\n\n", event)
				}
			}
		} else {
			if event.Event == futures.UserDataEventTypeListenKeyExpired {
				s.lineService.Notify(fmt.Sprintf("`Event:  🔴 KeyExpired 🔴, Time: %d", event.Time))
				return
			}

			s.lineService.Notify(fmt.Sprintf("`#4 Event: %s, Time: %d, Commission: %s", event.Event, event.Time, event.OrderTradeUpdate.Commission))
			log.Printf("### 4 ###: %+v\n\n", event)
		}
	}

	errHandler := func(err error) {
		log.Println("errHandler: ", err)
	}

	_, _, err = futures.WsUserDataServe(listenKey, wsHandler, errHandler)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (s *service) calculateRealizedPnl() (*models.CalculateRealizedPnl, error) {

	var profit, loss, commission float64
	startTime := utils.EpochStartDate(time.Now()).UnixMilli()
	endTime := utils.EpochEndDate(time.Now()).UnixMilli()

	trades, err := s.client.NewListAccountTradeService().StartTime(startTime).EndTime(endTime).Do(context.Background())
	if err != nil {
		return nil, err
	}

	for _, v := range trades {
		var realizedPnl, com float64
		if f, err := strconv.ParseFloat(v.RealizedPnl, 64); err == nil {
			realizedPnl = f
		}

		if f, err := strconv.ParseFloat(v.Commission, 64); err == nil {
			com = f
		}

		if realizedPnl > 0 {
			profit += realizedPnl
			commission += com
		} else if realizedPnl < 0 {
			loss += realizedPnl
			commission += com
		}
	}

	return &models.CalculateRealizedPnl{
		Profit:     profit,
		Loss:       loss,
		Commission: commission,
		NetProfit:  (profit - loss) - commission,
	}, nil

}

func (s *service) startScheduler() {
	log.Println("Start scheduler...")
	// gocron.Every(1).Day().At("10:30").Do(task)
	err := s.scheduler.Every(1).Day().At("23:59").Do(func() {

		realizedPnl, err := s.calculateRealizedPnl()
		if err != nil {
			log.Println("RealizedPnl: ", err)
		}
		// 🚸🎏🧬🧪
		msg2 := fmt.Sprintf(`🔰 DAILY REALIZED PNL
กำไร: %.2f
ขาดทุน: %.2f
Commission: %.2f
กำไรรวมวันนี้ %.2f`,
			realizedPnl.Profit,
			realizedPnl.Loss,
			realizedPnl.Commission,
			realizedPnl.NetProfit,
		)

		s.lineService.Notify(msg2)
	})
	if err != nil {
		log.Println("startScheduler", err)
	}

	err = s.scheduler.Every(30).Minutes().Do(func() {
		s.lineService.Notify("🧪 extend listenKey'" + s.listenKey)
		err := s.client.NewKeepaliveUserStreamService().ListenKey(s.listenKey).Do(context.Background())
		if err != nil {
			log.Println(err)
		}
	})

	// err = s.client.NewKeepaliveUserStreamService().ListenKey(listenKey).Do(context.Background())
	// if err != nil {
	// 	log.Println(err)
	// 	return err
	// }

	// Start all the pending jobs
	<-s.scheduler.Start()
	log.Println(err)
}
