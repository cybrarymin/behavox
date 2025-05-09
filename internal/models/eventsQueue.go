package data

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
)

var (
	CmdEventQueueSize int64
)

type EventQueue struct {
	Capacity int64
	Events   chan Event
}

func NewEventQueue() *EventQueue {
	eq := make(chan Event, CmdEventQueueSize)
	return &EventQueue{
		Capacity: int64(CmdEventQueueSize),
		Events:   eq,
	}
}

/*
PutEvent function will get an event and add that to the event queue
*/
func (eq *EventQueue) PutEvent(ctx context.Context, event Event) error {
	_, span := otel.Tracer("EventQueue.PutEvent.Tracer").Start(ctx, "EventQueue.PutEvent.Span")
	defer span.End()

	if len(eq.Events) == cap(eq.Events) {
		return errors.New("event queue is full")
	}

	// Set the enqueue time if the event implements BaseEvent
	if baseEvent, ok := event.(*EventLog); ok {
		baseEvent.BaseEvent.EnqueueTime = time.Now()
	} else if baseEvent, ok := event.(*EventMetric); ok {
		baseEvent.BaseEvent.EnqueueTime = time.Now()
	}

	// Append to the Queue
	eq.Events <- event
	return nil
}

/*
GetEvent function will get an event out the queue completely in FIFO mode and and shrinks the eventQueue
*/
func (eq *EventQueue) GetEvent(ctx context.Context) Event {
	// Check if the queue is empty
	if len(eq.Events) == 0 {
		return nil
	}
	_, span := otel.Tracer("EventQueue.GetEvent.Tracer").Start(ctx, "EventQueue.GetEvent.Span")
	defer span.End()
	span.AddEvent("Event removed from queue")
	return <-eq.Events
}

/*
Size function will get the size of current Queue
*/
func (eq *EventQueue) Size(ctx context.Context) int {
	_, span := otel.Tracer("EventQueue.Size.Tracer").Start(ctx, "EventQueue.Size.Span")
	defer span.End()
	return len(eq.Events)
}
