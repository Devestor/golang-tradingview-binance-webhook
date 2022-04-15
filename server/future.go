package server

import (
	"context"
	"fmt"
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

	data := &models.RequestFromTradingViewAlert{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}
	command := parseRawCommand(data.Mode)

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

func parseRawCommand(rawCommand string) *models.Command {

	arr := strings.Split(rawCommand, "_")

	c := &models.Command{}

	c.Symbol = arr[0]
	c.IsTP = arr[3] == "true"
	c.IsSL = arr[4] == "true"

	// Side
	if arr[1] == string(futures.PositionSideTypeLong) {
		c.Side = futures.PositionSideTypeLong
	} else if arr[1] == string(futures.PositionSideTypeShort) {
		c.Side = futures.PositionSideTypeShort
	}

	// AmountUSD
	intAmountUSD, _ := strconv.Atoi(arr[2])
	c.AmountUSD = int64(intAmountUSD)

	return c
}
