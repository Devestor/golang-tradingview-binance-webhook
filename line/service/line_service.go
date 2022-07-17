package service

import (
	"bytes"
	"net/http"
	"net/url"
	"tradingview-binance-webhook/client"
	"tradingview-binance-webhook/line"

	"github.com/sirupsen/logrus"
)

type lineService struct {
	notifyToken string
	httpClient  *client.Client
}

// NewLineService is the line services
func NewLineService(notifyToken string, httpClient *client.Client) line.Service {
	return &lineService{
		notifyToken: "Bearer " + notifyToken,
		httpClient:  httpClient,
	}
}

func (s *lineService) Notify(meessage string) error {
	uri := "https://notify-api.line.me/api/notify"
	data := url.Values{}
	data.Set("message", meessage)

	req, _ := http.NewRequest("POST", uri, bytes.NewBufferString(data.Encode()))

	// Headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	req.Header.Set("Authorization", s.notifyToken)

	resp, _ := s.httpClient.Do(req, nil)
	if resp.StatusCode == 200 {
		logrus.Info("Line success sending message")
	} else {
		logrus.Info("Line failed sending message")
	}
	return nil
}
