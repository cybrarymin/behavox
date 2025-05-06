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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type Envelope map[string]interface{}

// WriteJson will write the data as response with desired http header and http status code
func WriteJson(ctx context.Context, w http.ResponseWriter, status int, data Envelope, headers http.Header) error {
	ctx, parentSpan := otel.Tracer("WriteJson.Tracer").Start(ctx, "WriteJson.Span")
	defer parentSpan.End()

	// Create a span for JSON encoding
	_, encodeSpan := otel.Tracer("WriteJson.Tracer").Start(ctx, "WriteJson.Encode")
	// considering bytes.Buffer instead of directly writing to the http.responseWriter to be able to segregate the error handling for json marshaling and write errors
	nBuffer := bytes.Buffer{}
	err := json.NewEncoder(&nBuffer).Encode(data)
	if err != nil {
		encodeSpan.RecordError(err)
		encodeSpan.SetStatus(codes.Error, "failed to serialize data into json format")
		encodeSpan.End()

		parentSpan.RecordError(err)
		parentSpan.SetStatus(codes.Error, "failed to serialize data into json format")
		return err
	}
	encodeSpan.SetAttributes(attribute.Int("encoded_bytes", nBuffer.Len()))
	encodeSpan.End()

	// Create a span for setting headers and writing response
	_, writeSpan := otel.Tracer("WriteJson.Tracer").Start(ctx, "WriteJson.WriteResponse")
	for key, value := range headers {
		w.Header()[key] = value
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	writeSpan.SetAttributes(attribute.Int("status_code", status))

	_, err = w.Write(nBuffer.Bytes())
	if err != nil {
		writeSpan.RecordError(err)
		writeSpan.SetStatus(codes.Error, "failed to write json data as a response")
		writeSpan.End()

		parentSpan.RecordError(err)
		parentSpan.SetStatus(codes.Error, "failed to write json data as a response")
		return err
	}

	writeSpan.SetStatus(codes.Ok, "successfully wrote response")
	writeSpan.End()

	parentSpan.SetStatus(codes.Ok, "successfully wrote JSON response")
	return nil
}

// ReadJson reads the json bytes from a requests and deserialize it in dst
func ReadJson[T any](ctx context.Context, w http.ResponseWriter, r *http.Request) (T, error) {
	ctx, parentSpan := otel.Tracer("ReadJson.Tracer").Start(ctx, "ReadJson.Span")
	defer parentSpan.End()

	var output, zero T

	// Create a span for setting up the reader with size limits
	_, setupSpan := otel.Tracer("ReadJson.Tracer").Start(ctx, "ReadJson.Setup")
	// Limit the amount of bytes accepted as post request body
	maxBytes := 1_048_576 // _ here is only for visual separator purpose and for int values go's compiler will ignore it.
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))
	dec := json.NewDecoder(r.Body)
	// Initialize the json.Decoder, and call the DisallowUnknownFields() method on it
	// before decoding. This means that if the JSON from the client now includes any
	// field which cannot be mapped to the target destination, the decoder will return
	// an error instead of just ignoring the field.
	dec.DisallowUnknownFields()
	setupSpan.SetAttributes(attribute.Bool("disallow_unknown_fields", true))
	setupSpan.SetAttributes(attribute.Int64("max_bytes", int64(maxBytes)))
	setupSpan.End()

	// Create a span for the actual JSON decoding
	_, decodeSpan := otel.Tracer("ReadJson.Tracer").Start(ctx, "ReadJson.Decode")
	err := dec.Decode(&output)
	if err == nil {
		decodeSpan.SetAttributes(attribute.Bool("decode_success", true))
	} else {
		decodeSpan.SetAttributes(attribute.Bool("decode_success", false))
		decodeSpan.SetAttributes(attribute.String("error_type", fmt.Sprintf("%T", err)))
		decodeSpan.SetAttributes(attribute.String("error_message", err.Error()))
	}
	decodeSpan.End()

	if err != nil {
		_, errorSpan := otel.Tracer("ReadJson.Tracer").Start(ctx, "ReadJson.ErrorHandling")
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		errorType := "unknown"

		switch {
		// This happens if we json syntax errors. having wrong commas or indentation or missing quotes
		case errors.As(err, &syntaxError):
			errorType = "syntax_error"
			err = fmt.Errorf("body contains badly-formed json (at character %d)", syntaxError.Offset)
			parentSpan.RecordError(err)
			parentSpan.SetStatus(codes.Error, "failed to read the json body")
			errorSpan.SetAttributes(attribute.String("error_type", errorType))
			errorSpan.SetAttributes(attribute.Int64("error_offset", syntaxError.Offset))
			errorSpan.End()
			return zero, err
		case errors.Is(err, io.ErrUnexpectedEOF):
			errorType = "unexpected_eof"
			var zero T
			err = errors.New("body contains badly-formed JSON")
			parentSpan.RecordError(err)
			parentSpan.SetStatus(codes.Error, "failed to read the json body")
			errorSpan.SetAttributes(attribute.String("error_type", errorType))
			errorSpan.End()
			return zero, err

		// This will happen if we try to unmarshal a json value of a type to a struct field that doesn't support that specific type
		case errors.As(err, &unmarshalTypeError):
			errorType = "type_error"
			if unmarshalTypeError.Field != "" {
				err = fmt.Errorf("invalid type used for the key %s", unmarshalTypeError.Field)
				errorSpan.SetAttributes(attribute.String("error_field", unmarshalTypeError.Field))
				parentSpan.RecordError(err)
				parentSpan.SetStatus(codes.Error, "failed to read the json body")
				errorSpan.SetAttributes(attribute.String("error_type", errorType))
				errorSpan.End()
				return zero, err
			}
			// if client provide completely different type of json. for example instead of json of object type it sends an array content json
			err = fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)
			parentSpan.RecordError(err)
			parentSpan.SetStatus(codes.Error, "failed to read the json body")
			errorSpan.SetAttributes(attribute.String("error_type", errorType))
			errorSpan.SetAttributes(attribute.Int64("error_offset", unmarshalTypeError.Offset))
			errorSpan.End()
			return zero, err

		// If the JSON contains a field which cannot be mapped to the target destination
		// then Decode() will now return an error message in the format "json: unknown
		// field "<n>"". We check for this, extract the field name from the error,
		// and interpolate it into our custom error message.
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			errorType = "unknown_field"
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field")
			err = fmt.Errorf("body contains unknown field %s", fieldName)
			parentSpan.RecordError(err)
			parentSpan.SetStatus(codes.Error, "failed to read the json body")
			errorSpan.SetAttributes(attribute.String("error_type", errorType))
			errorSpan.SetAttributes(attribute.String("unknown_field", fieldName))
			errorSpan.End()
			return zero, err

		// If the request body exceeds 1MB in size the decode will now fail with the
		// error "http: request body too large". There is an open issue about turning
		// this into a distinct error type at https://github.com/golang/go/issues/30715.
		case err.Error() == "http: request body too large":
			errorType = "body_too_large"
			err = fmt.Errorf("body must not be larger than %d bytes", maxBytes)
			parentSpan.RecordError(err)
			parentSpan.SetStatus(codes.Error, "failed to read the json body")
			errorSpan.SetAttributes(attribute.String("error_type", errorType))
			errorSpan.SetAttributes(attribute.Int64("max_bytes", int64(maxBytes)))
			errorSpan.End()
			return zero, err

		// Error will happen if we pass invalid type to json.Decode function. we should always pass a pointer otherwise it will give us error
		case errors.As(err, &invalidUnmarshalError):
			errorType = "invalid_unmarshal"
			errorSpan.SetAttributes(attribute.String("error_type", errorType))
			errorSpan.End()
			panic(err)
		case errors.Is(err, io.EOF):
			errorType = "empty_body"
			err = errors.New("json body must not be empty")
			parentSpan.RecordError(err)
			parentSpan.SetStatus(codes.Error, "failed to read the json body")
			errorSpan.SetAttributes(attribute.String("error_type", errorType))
			errorSpan.End()
			return zero, err
		default:
			errorSpan.SetAttributes(attribute.String("error_type", errorType))
			errorSpan.End()
			return zero, err
		}
	}

	// Create a span for checking for multiple JSON values
	_, validateSpan := otel.Tracer("ReadJson.Tracer").Start(ctx, "ReadJson.ValidateSingleValue")
	// by default decode method of json package will read json values one by one.
	// If the request body only contained a single JSON value this will
	// return an io.EOF error. So if we get anything else, we know that there is
	// additional data in the request body and we return our own custom error message.
	err = dec.Decode(&struct{}{})
	if err == io.EOF {
		validateSpan.SetAttributes(attribute.Bool("single_value", true))
		validateSpan.End()
	} else {
		validateSpan.SetAttributes(attribute.Bool("single_value", false))
		validateSpan.End()
		err = errors.New("body must only contain a single json value")
		parentSpan.RecordError(err)
		parentSpan.SetStatus(codes.Error, "failed to read the json body")
		return zero, err
	}

	parentSpan.SetStatus(codes.Ok, "successfully parsed JSON")
	return output, nil
}
