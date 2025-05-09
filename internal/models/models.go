package data

type Models struct {
	EventQueue *EventQueue
}

func NewModels(eq *EventQueue, em *EventMetric, el *EventLog) *Models {
	return &Models{
		EventQueue: eq,
	}
}
