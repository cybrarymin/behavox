package data

import "github.com/google/uuid"

type EventModel struct {
	Id    uuid.UUID `json:"id"`
	Event string    `json:"event"`
}

func NewEvent(event string) *EventModel {
	return &EventModel{
		Id:    uuid.New(),
		Event: event,
	}
}
