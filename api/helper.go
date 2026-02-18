package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/thisisjab/logzilla/fault"
)

type apiResponse struct {
	Success  bool           `json:"success"`
	Message  string         `json:"message,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func (s *server) readJson(w http.ResponseWriter, r *http.Request, dst any) error { //nolint:unused
	maxBytes := 1_048_576
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		var maxBytesError *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxError):
			return fault.New(fault.BadInputCode, fmt.Sprintf("Body contains badly-formed JSON at character %d.", syntaxError.Offset))

		case errors.Is(err, io.ErrUnexpectedEOF):

			return fault.New(fault.BadInputCode, "Body contains badly-formed JSON.")

		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fault.New(fault.BadInputCode, "").WithMetadata(fault.FieldErrorsMetadata{
					unmarshalTypeError.Field: []string{fmt.Sprintf("Expected type %s.", unmarshalTypeError.Type.String())},
				})
			}

			return fault.New(fault.BadInputCode, fmt.Sprintf("Body contains badly-formed JSON at character %d.", unmarshalTypeError.Offset))

		case errors.Is(err, io.EOF):
			return fault.New(fault.BadInputCode, "Body cannot be empty.")

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			fieldName = strings.ReplaceAll(fieldName, "\"", "")

			return fault.New(fault.BadInputCode, "").WithMetadata(fault.FieldErrorsMetadata{
				fieldName: []string{"Key is unknown."},
			})

		case errors.As(err, &maxBytesError):
			return fault.New(fault.BadInputCode, fmt.Sprintf("Body must not be larger than %d bytes.", maxBytesError.Limit))

		case errors.As(err, &invalidUnmarshalError):
			panic(err)

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return fault.New(fault.BadInputCode, "Body must only contain a single JSON value.")
	}

	return nil
}

func (s *server) writeJson(w http.ResponseWriter, status int, data apiResponse, headers http.Header) error {
	js, err := json.Marshal(data)
	if err != nil {
		return err
	}

	js = append(js, '\n')
	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js) //nolint:errcheck

	return nil
}
