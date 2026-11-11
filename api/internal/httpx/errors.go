package httpx

import (
	"errors"
	"log"
	"net/http"
)

// Error is an API error rendered as {"error":{"code","message","fields"}}.
type Error struct {
	Status  int
	Code    string
	Message string
	Fields  map[string]string
}

func (e Error) Error() string { return e.Code + ": " + e.Message }

type errBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// Handler is an http handler that may return an error.
type Handler func(w http.ResponseWriter, r *http.Request) error

// H adapts a Handler to http.HandlerFunc, writing errors in the global format.
func H(h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := h(w, r)
		if err == nil {
			return
		}
		var he Error
		var hp *Error
		switch {
		case errors.As(err, &he):
		case errors.As(err, &hp) && hp != nil:
			he = *hp
		default:
			he = Error{}
		}
		if he.Status != 0 {
			JSON(w, he.Status, map[string]any{"error": errBody{he.Code, he.Message, he.Fields}})
			return
		}
		log.Printf("internal error: %s %s: %v", r.Method, r.URL.Path, err)
		JSON(w, http.StatusInternalServerError, map[string]any{
			"error": errBody{Code: "internal", Message: "internal server error"},
		})
	}
}
