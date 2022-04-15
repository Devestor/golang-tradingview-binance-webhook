package models

import "net/http"

type RequestFromTradingViewAlert struct {
	Mode string `json:"m"`
}

func (o *RequestFromTradingViewAlert) Bind(r *http.Request) error {
	return nil
}

func (o *RequestFromTradingViewAlert) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}
