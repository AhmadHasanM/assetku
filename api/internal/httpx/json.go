package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
)

// MaxJSONBytes caps JSON request bodies. File uploads (imports) set their own,
// larger limit and do not go through Decode.
const MaxJSONBytes = 1 << 20

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Decode parses the JSON request body into v; failure yields 400 invalid_json,
// a body over MaxJSONBytes yields 413 body_too_large.
func Decode(r *http.Request, v any) error {
	body := http.MaxBytesReader(nil, r.Body, MaxJSONBytes)
	if err := json.NewDecoder(body).Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return Error{Status: http.StatusRequestEntityTooLarge, Code: "body_too_large", Message: "request body too large"}
		}
		return Error{Status: http.StatusBadRequest, Code: "invalid_json", Message: "invalid JSON body"}
	}
	return nil
}
