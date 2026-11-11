package httpx

import (
	"encoding/json"
	"net/http"
)

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Decode parses the JSON request body into v; failure yields 400 invalid_json.
func Decode(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return Error{Status: http.StatusBadRequest, Code: "invalid_json", Message: "invalid JSON body"}
	}
	return nil
}
