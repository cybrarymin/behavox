package data

import (
	"fmt"
	"time"
)

const (
	EventTypeMetric = "metric"
	EventTypeLog    = "log"
)

/*
Event is an interface for all event types
*/
type Event interface {
	GetMetadata() map[string]interface{}
	GetCommonMetadata() map[string]interface{}
	GetEventID() string
}

/*
BaseEvent implements common functionality for all events
*/
type BaseEvent struct {
	EventID     string
	Timestamp   string
	ThreadID    int
	EnqueueTime time.Time // Time when the event was added to the queue
}

/*
NewBaseEvent creates a new BaseEvent with the given event ID
*/
func NewBaseEvent(eventID string) *BaseEvent {
	return &BaseEvent{
		EventID:     eventID,
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
		ThreadID:    0,
		EnqueueTime: time.Time{}, // Will be set when added to queue
	}
}

/*
GetEventID returns the event ID
*/
func (b BaseEvent) GetEventID() string {
	return b.EventID
}

/*
GetCommonMetadata returns common metadata for all event types
*/
func (b BaseEvent) GetCommonMetadata() map[string]interface{} {
	return map[string]interface{}{
		"event_id":   b.EventID,
		"timestamp":  b.Timestamp,
		"thread_id":  b.ThreadID,
		"event_type": fmt.Sprintf("%T", b),
	}
}

/*
EventMetric represents a metric event with a numerical value
*/
type EventMetric struct {
	*BaseEvent
	Value float64
}

/*
NewEventMetric creates a new EventMetric
*/
func NewEventMetric(eventID string, value float64) *EventMetric {
	return &EventMetric{
		BaseEvent: NewBaseEvent(eventID),
		Value:     value,
	}
}

/*
GetMetadata returns metadata for EventMetric
*/
func (e EventMetric) GetMetadata() map[string]interface{} {
	metadata := e.GetCommonMetadata()
	metadata["value"] = e.Value
	return metadata
}

/*
EventLog represents a log event with a level and message
*/
type EventLog struct {
	*BaseEvent
	Level   string
	Message string
}

/*
NewEventLog creates a new EventLog
*/
func NewEventLog(eventID string, level string, message string) *EventLog {
	return &EventLog{
		BaseEvent: NewBaseEvent(eventID),
		Level:     level,
		Message:   message,
	}
}

/*
GetMetadata returns metadata for EventLog
*/
func (e EventLog) GetMetadata() map[string]interface{} {
	metadata := e.GetCommonMetadata()
	metadata["level"] = e.Level
	metadata["message"] = e.Message
	return metadata
}
