package server

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"tradingview-binance-webhook/future"
	"tradingview-binance-webhook/models"
)

type futureHandler struct {
	s future.Service
}

func (h *futureHandler) router() chi.Router {
	r := chi.NewRouter()

	r.Post("/tradingview", h.receiveCommandFromTradingView)

	return r
}

func (h *futureHandler) receiveCommandFromTradingView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	strReqBody := string(reqBody)
	command, err := parseRawCommand(strReqBody)
	if err != nil {
		log.Println(err)
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	log.Printf("Command: %s\n Symbol: %s, Side: %s, Amount: %d, TP: %t, SL: %t, CheckWL: %t\n", strReqBody, command.Symbol, command.Side, command.AmountUSD, command.IsTP, command.IsSL, command.IsCheckWL)

	switch command.Side {
	case futures.PositionSideTypeLong:
		err := h.s.Long(command)
		if err != nil {
			log.Println(err)
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		render.Respond(w, r, SuccessResponse(nil, "success"))
	case futures.PositionSideTypeShort:
		err := h.s.Short(command)
		if err != nil {
			log.Println(err)
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		render.Respond(w, r, SuccessResponse(nil, "success"))
	default:
		fmt.Printf("%s.\n", command.Side)
	}

	render.Respond(w, r, SuccessResponse(nil, "success"))
}

func parseRawCommand(rawCommand string) (*models.Command, error) {
	arr := strings.Split(rawCommand, "_")

	c := &models.Command{}

	if len(arr) < 1 {
		return nil, errors.New("raw command error")
	}

	c.Symbol = arr[0] // 1

	if len(arr) >= 4 {
		c.IsTP = arr[3] == "true" // 4
	}

	if len(arr) >= 5 {
		c.IsSL = arr[4] == "true" // 5
	}

	if len(arr) >= 6 {
		c.IsCheckWL = arr[5] == "true" // 6
	}

	// Side
	if strings.ToUpper(arr[1]) == strings.ToUpper(string(futures.PositionSideTypeLong)) {
		c.Side = futures.PositionSideTypeLong
	} else if strings.ToUpper(arr[1]) == strings.ToUpper(string(futures.PositionSideTypeShort)) {
		c.Side = futures.PositionSideTypeShort
	}

	// AmountUSD
	intAmountUSD, _ := strconv.Atoi(arr[2])
	c.AmountUSD = int64(intAmountUSD)

	return c, nil
}
