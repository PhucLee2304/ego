package httpx

import (
	"ego/platform/logger"
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

type ErrorResponse struct {
	Error string `json:"error"`
}

func Error(w http.ResponseWriter, code int, message string) {
	if code > 499 {
		logger.Log.Error().Int("code", code).Msg("Internal server error")
	}

	JSON(w, code, ErrorResponse{
		Error: message,
	})
}

func DecodeJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return err
	}

	if err := validate.Struct(v); err != nil {
		return err
	}

	return nil
}

func JSON(w http.ResponseWriter, code int, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		logger.Log.Error().Err(err).Msg("[INTERNAL_SERVER_ERROR] Error marshalling response")
		w.WriteHeader(500)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}
