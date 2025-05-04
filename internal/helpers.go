package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

type Envelope map[string]interface{}

// WriteJson will write the data as response with desired http header and http status code
func WriteJson(ctx context.Context, w http.ResponseWriter, status int, data Envelope, headers http.Header) error {
	_, span := otel.Tracer("ReadJson.Tracer").Start(ctx, "ReadJson.Span")
	defer span.End()

	// considering bytes.Buffer instead of directly writing to the http.responseWriter to be able to segregate the error handling for json marshaling and write errors
	nBuffer := bytes.Buffer{}
	err := json.NewEncoder(&nBuffer).Encode(data)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to serialize data into json format")
		return err
	}

	for key, value := range headers {
		w.Header()[key] = value
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(nBuffer.Bytes())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to write json data as a response")
		return err
	}
	return nil
}

// ReadJson reads the json bytes from a requests and deserialize it in dst
func ReadJson[T any](ctx context.Context, w http.ResponseWriter, r *http.Request) (T, error) {
	_, span := otel.Tracer("ReadJson.Tracer").Start(ctx, "ReadJson.Span")
	defer span.End()

	var output, zero T
	// Limit the amount of bytes accepted as post request body
	maxBytes := 1_048_576 // _ here is only for visual separator purpose and for int values go's compiler will ignore it.
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))
	dec := json.NewDecoder(r.Body)
	// Initialize the json.Decoder, and call the DisallowUnknownFields() method on it
	// before decoding. This means that if the JSON from the client now includes any
	// field which cannot be mapped to the target destination, the decoder will return
	// an error instead of just ignoring the field.
	dec.DisallowUnknownFields()
	err := dec.Decode(&output)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		switch {
		// This happens if we json syntax errors. having wrong commas or indentation or missing quotes
		case errors.As(err, &syntaxError):
			err = fmt.Errorf("body contains badly-formed json (at character %d)", syntaxError.Offset)
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to read the json body")
			return zero, err
		case errors.Is(err, io.ErrUnexpectedEOF):
			var zero T
			err = errors.New("body contains badly-formed JSON")
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to read the json body")
			return zero, err

		// This will happen if we try to unmarshal a json value of a type to a struct field that doesn't support that specific type
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				err = fmt.Errorf("invalid type used for the key %s", unmarshalTypeError.Field)
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to read the json body")
				return zero, err
			}
			// if client provide completely different type of json. for example instead of json of object type it sends an array content json
			err = fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to read the json body")
			return zero, err

		// If the JSON contains a field which cannot be mapped to the target destination
		// then Decode() will now return an error message in the format "json: unknown
		// field "<name>"". We check for this, extract the field name from the error,
		// and interpolate it into our custom error message.
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field")
			err = fmt.Errorf("body contains unknown field %s", fieldName)
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to read the json body")
			return zero, err

		// If the request body exceeds 1MB in size the decode will now fail with the
		// error "http: request body too large". There is an open issue about turning
		// this into a distinct error type at https://github.com/golang/go/issues/30715.
		case err.Error() == "http: request body too large":
			err = fmt.Errorf("body must not be larger than %d bytes", maxBytes)
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to read the json body")
			return zero, err

		// Error will happen if we pass invalid type to json.Decode function. we should always pass a pointer otherwise it will give us error
		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		case errors.Is(err, io.EOF):
			err = errors.New("json body must not be empty")
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to read the json body")
			return zero, err
		default:
			return zero, err
		}
	}

	// by default decode method of json package will read json values one by one.
	// If the request body only contained a single JSON value this will
	// return an io.EOF error. So if we get anything else, we know that there is
	// additional data in the request body and we return our own custom error message.
	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		err = errors.New("body must only contain a single json value")
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to read the json body")
		return zero, err
	}
	return output, nil
}
