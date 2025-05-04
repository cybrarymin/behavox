package api

import (
	"fmt"
	"net/http"

	helpers "github.com/cybrarymin/behavox/internal"
	data "github.com/cybrarymin/behavox/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type EventCreateReq struct {
	Event string `json:"event"`
}

func NewEventCreateReq(eventMessage string) *EventCreateReq {
	return &EventCreateReq{
		Event: eventMessage,
	}
}

type EventCreateRes struct {
	Id string `json:"id"`
}

func NewEventCreateRes(eventId string) *EventCreateRes {
	return &EventCreateRes{
		Id: eventId,
	}
}

func (api *ApiServer) createEventHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("createEventHandler.Tracer").Start(r.Context(), "createEventHandler.Span")
	defer span.End()

	nReq, err := helpers.ReadJson[EventCreateReq](ctx, w, r)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid input")
		api.badRequestResponse(w, r, err)
		return
	}

	nVal := helpers.NewValidator()
	nVal.Check(nReq.Event != "", "event", "shouldn't be an empty string")
	if !nVal.Valid() {
		for key, errString := range nVal.Errors {
			err := fmt.Errorf("%s message %s", key, errString)
			span.RecordError(err)
		}
		span.SetStatus(codes.Error, "invalid input")
		api.failedValidationResponse(w, r, nVal.Errors)
		return
	}

	nEvent := data.NewEvent(nReq.Event)
	span.SetAttributes(attribute.String("event.id", nEvent.Id.String()))
	/*
		Send a request to the Queue service
	*/

	api.Logger.Info().
		Str("event_id", nEvent.Id.String()).
		Str("event_message", nEvent.Event).Msg("New event created")

	nRes := NewEventCreateRes(nEvent.Id.String())
	err = helpers.WriteJson(ctx, w, http.StatusOK, helpers.Envelope{"result": nRes}, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to write the response for the client")
		api.serverErrorResponse(w, r, err)
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

	/*
		Send a request to the Queue service to get response
	*/
	queueCurrentSize := 1000
	api.Logger.Info().Msg("fetched the event queue size")
	nRes := NewEventStatsGetRes(uint64(queueCurrentSize))
	err := helpers.WriteJson(ctx, w, http.StatusOK, helpers.Envelope{"result": nRes}, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to write the response for the client")
		api.serverErrorResponse(w, r, err)
		return
	}
}
