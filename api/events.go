package api

import (
	"fmt"
	"net/http"

	helpers "github.com/cybrarymin/behavox/internal"
	data "github.com/cybrarymin/behavox/internal/models"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

type EventCreateReq struct {
	Event struct {
		EventType string   `json:"event_type"`
		EventID   string   `json:"event_id"`
		Value     *float64 `json:"value,omitempty"`
		Level     *string  `json:"level,omitempty"`
		Message   *string  `json:"message,omitempty"`
	} `json:"event"`
}

func NewEventCreateReq(eventType string, eventID string, value *float64, level *string, message *string) *EventCreateReq {
	return &EventCreateReq{
		Event: struct {
			EventType string   "json:\"event_type\""
			EventID   string   "json:\"event_id\""
			Value     *float64 "json:\"value,omitempty\""
			Level     *string  "json:\"level,omitempty\""
			Message   *string  "json:\"message,omitempty\""
		}{

			EventType: eventType,
			EventID:   eventID,
			Value:     value,
			Level:     level,
			Message:   message,
		},
	}
}

type EventCreateRes struct {
	Event struct {
		EventType string   `json:"event_type"`
		EventID   string   `json:"event_id"`
		Value     *float64 `json:"value,omitempty"`
		Level     *string  `json:"level,omitempty"`
		Message   *string  `json:"message,omitempty"`
	} `json:"event"`
}

func NewEventCreateRes(eventType string, eventID string, value *float64, level *string, message *string) *EventCreateRes {
	return &EventCreateRes{
		Event: struct {
			EventType string   "json:\"event_type\""
			EventID   string   "json:\"event_id\""
			Value     *float64 "json:\"value,omitempty\""
			Level     *string  "json:\"level,omitempty\""
			Message   *string  "json:\"message,omitempty\""
		}{
			EventType: eventType,
			EventID:   eventID,
			Value:     value,
			Level:     level,
			Message:   message,
		},
	}
}

func (api *ApiServer) createEventHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("createEventHandler.Tracer").Start(r.Context(), "createEventHandler.Span")
	defer span.End()

	// Reading the request body
	nReq, err := helpers.ReadJson[EventCreateReq](ctx, w, r)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid input")
		api.badRequestResponse(w, r, err)
		return
	}

	// Input validation
	nVal := helpers.NewValidator()
	_, err = uuid.Parse(nReq.Event.EventID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid input")
		api.badRequestResponse(w, r, fmt.Errorf("event_id should be a valid uuid"))
		return
	}
	nVal.Check(nReq.Event.EventType != "", "event_type", "shouldn't be nil")
	validEventTypes := []string{data.EventTypeLog, data.EventTypeMetric}
	nVal.Check(helpers.In(nReq.Event.EventType, validEventTypes...), "event_type", "invalid")

	switch nReq.Event.EventType {
	case data.EventTypeLog:
		if nReq.Event.Value != nil {
			api.badRequestResponse(w, r, fmt.Errorf("body contains unknown field \"value\""))
			return
		}
		nVal.Check(nReq.Event.Level != nil, "level", "shouldn't be nil")
		nVal.Check(nReq.Event.Message != nil, "message", "shouldn't be nil")

	case data.EventTypeMetric:
		switch {
		case nReq.Event.Level != nil:
			api.badRequestResponse(w, r, fmt.Errorf("body contains unknown field \"level\""))
			return
		case nReq.Event.Message != nil:
			api.badRequestResponse(w, r, fmt.Errorf("body contains unknown field \"message\""))
			return
		}
		nVal.Check(nReq.Event.Value != nil, "value", "shouldn't be nil")
	}

	if !nVal.Valid() {
		for key, errString := range nVal.Errors {
			err := fmt.Errorf("%s message %s", key, errString)
			span.RecordError(err)
		}
		span.SetStatus(codes.Error, "invalid input")
		api.failedValidationResponse(w, r, nVal.Errors)
		return
	}

	var nEvent data.Event
	switch nReq.Event.EventType {
	case data.EventTypeLog:
		api.Logger.Info().
			Str("event_id", nReq.Event.EventID).
			Str("event_type", nReq.Event.EventType).
			Str("message", *nReq.Event.Message).
			Str("level", *nReq.Event.Level).
			Msg("creating new event")

		nEvent = data.NewEventLog(nReq.Event.EventID, *nReq.Event.Level, *nReq.Event.Message)
		span.AddEvent("new log event created")

	case data.EventTypeMetric:
		api.Logger.Info().
			Str("event_id", nReq.Event.EventID).
			Str("event_type", nReq.Event.EventType).
			Float64("value", *nReq.Event.Value).
			Msg("creating new event")

		nEvent = data.NewEventMetric(nReq.Event.EventID, *nReq.Event.Value)
		span.AddEvent("new metric event created")
	}

	err = api.models.EventQueue.PutEvent(ctx, nEvent)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to add new event into the queue")
		api.eventQueueFullResponse(w, r)
	}

	nRes := NewEventCreateRes(nReq.Event.EventType, nReq.Event.EventID, nReq.Event.Value, nReq.Event.Level, nReq.Event.Message)
	err = helpers.WriteJson(ctx, w, http.StatusCreated, helpers.Envelope{"event": nRes}, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to write the response for the client")
		api.serverErrorResponse(w, r, err)
		return
	}
}

type EventStatsGetRes struct {
	Queue_size uint64 `json:"queue_size"`
}

func NewEventStatsGetRes(qSize uint64) *EventStatsGetRes {
	return &EventStatsGetRes{
		Queue_size: qSize,
	}
}

func (api *ApiServer) GetEventStatsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("GetEventStatsHandler.Tracer").Start(r.Context(), "GetEventStatsHandler.Span")
	defer span.End()

	// Send a request to the Queue service to get response

	queueCurrentSize := api.models.EventQueue.Size(ctx)

	api.Logger.Info().
		Int64("queue_size", int64(queueCurrentSize)).
		Str("remote_addr", r.RemoteAddr).
		Msg("fetched the event queue size")

	nRes := NewEventStatsGetRes(uint64(queueCurrentSize))
	err := helpers.WriteJson(ctx, w, http.StatusOK, helpers.Envelope{"result": nRes}, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to write the response for the client")
		api.serverErrorResponse(w, r, err)
		return
	}
}
