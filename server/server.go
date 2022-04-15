package server

import (
	"log"
	"net/http"
	"strings"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"

	"tradingview-binance-webhook/constants"
	"tradingview-binance-webhook/future"
)

type Server struct {
	router    chi.Router
	client    *futures.Client
	futureSvc future.Service
}

func New(
	client *futures.Client,
	futureSvc future.Service,
) *Server {
	s := &Server{
		client:    client,
		futureSvc: futureSvc,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Group(func(r chi.Router) {

		r.Route("/v1", func(r chi.Router) {
			futureSvcSvc := futureHandler{s.futureSvc}
			r.Mount("/", futureSvcSvc.router())
		})
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("welcome"))
	})

	s.router = r

	walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		route = strings.Replace(route, "/*/", "/", -1)
		log.Printf("%s %s\n", method, route) // Walk and print out all routes
		return nil
	}

	if err := chi.Walk(r, walkFunc); err != nil {
		log.Panicf("Logging err: %s\n", err.Error()) // panic if there is an error
	}

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

type ErrResponse struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	StatusText string `json:"status,omitempty"`  // user-level status message
	AppCode    int64  `json:"code,omitempty"`    // application-specific error code
	Message    string `json:"message,omitempty"` // application-level error message, for debugging
}

func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

func ErrInvalidRequest(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusBadRequest,
		AppCode:        constants.CodeError,
		// StatusText:     "Invalid request.",
		Message: err.Error(),
	}
}

type ApiResponse struct {
	HTTPStatusCode int `json:"-"` // http response status code

	AppCode int64       `json:"code,omitempty"` // application-specific error code
	Data    interface{} `json:"data,omitempty"` // application-specific error code
	Message string      `json:"message"`        // application-level error message, for debugging
}

func (e *ApiResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

func SuccessResponse(data interface{}, msg string) render.Renderer {
	return &ApiResponse{
		HTTPStatusCode: http.StatusOK,
		AppCode:        constants.CodeSuccess,
		Data:           data,
		Message:        msg,
	}
}
