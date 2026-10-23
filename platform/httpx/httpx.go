package httpx

import (
	"ego/platform/validatorx"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gorilla/schema"
)

var (
	decoder      = schema.NewDecoder()
	queryEncoder = schema.NewEncoder()
	validate     = validatorx.New()
)

func DecodeJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return err
	}
	return validate.Struct(v)
}

func DecodeQuery(r *http.Request, v any) error {
	if err := decoder.Decode(v, r.URL.Query()); err != nil {
		return err
	}
	return validate.Struct(v)
}

func ParsePathID(r *http.Request, name string) (uint, error) {
	id, err := strconv.ParseUint(r.PathValue(name), 10, 32)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

func EncodeQuery(v any) (url.Values, error) {
	values := url.Values{}
	err := queryEncoder.Encode(v, values)
	return values, err
}
