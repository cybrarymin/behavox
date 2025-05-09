package worker

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	observ "github.com/cybrarymin/behavox/api/observability"
	helpers "github.com/cybrarymin/behavox/internal"
	data "github.com/cybrarymin/behavox/internal/models"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var (
	CmdProcessedEventFile  string
	CmdmaxWorkerGoroutines int
)

type Worker struct {
	wg         sync.WaitGroup
	Logger     *zerolog.Logger
	EventQueue *data.EventQueue
	Ctx        context.Context
	Cancel     context.CancelFunc
	fileLock   sync.Mutex
}

func NewWorker(logger *zerolog.Logger, eq *data.EventQueue, ctx context.Context) *Worker {
	ctx, cancel := context.WithCancel(ctx)
	return &Worker{
		Logger:     logger,
		EventQueue: eq,
		Cancel:     cancel,
		Ctx:        ctx,
	}
}

func (w *Worker) Run(ctx context.Context) {
	w.Logger.Info().Msgf("starting the worker process in the background with %d number of threads for processing", CmdmaxWorkerGoroutines)

	runCtx := w.Ctx
	w.wg.Add(1)
	defer w.wg.Done()

	// make a semaphore pattern to impede having lot's of goroutines
	semaphore := make(chan struct{}, CmdmaxWorkerGoroutines)

	for {
		select {
		case nEvent := <-w.EventQueue.Events:
			w.wg.Add(1)

			semaphore <- struct{}{} // if the number of goroutines we are running to process each event exceeds 10 this will wait until one goroutine freeUp
			go func(event data.Event) {
				defer w.wg.Done()
				defer func() { <-semaphore }() // read from semaphore

				spanCtx, span := otel.Tracer("Worker.Tracer").Start(ctx, "Worker.Span")
				var EventType string
				switch event.(type) {
				case *data.EventLog:
					EventType = "log"
				case *data.EventMetric:
					EventType = "metric"
				}

				// Measure queue wait time (time from enqueue to processing)
				var queueWaitTime float64
				if baseEvent, ok := event.(*data.EventLog); ok && !baseEvent.BaseEvent.EnqueueTime.IsZero() {
					queueWaitTime = time.Since(baseEvent.BaseEvent.EnqueueTime).Seconds()
					observ.PromEventQueueWaitTime.WithLabelValues(EventType).Observe(queueWaitTime)
				} else if baseEvent, ok := event.(*data.EventMetric); ok && !baseEvent.BaseEvent.EnqueueTime.IsZero() {
					queueWaitTime = time.Since(baseEvent.BaseEvent.EnqueueTime).Seconds()
					observ.PromEventQueueWaitTime.WithLabelValues(EventType).Observe(queueWaitTime)
				}

				// Capture the start time for event processing duration
				eventProcessingStart := time.Now()

				w.Logger.Info().
					Str("event_id", event.GetEventID()).
					Msg("worker started processing the event")

				err := w.processEvent(spanCtx, event)
				if err != nil {
					w.Logger.Error().Err(err).
						Str("event_id", event.GetEventID()).
						Msg("event processing failed")

					time.Sleep(2 * time.Second) // wait for two second and reprocess the event
					// Check if context is cancelled before retry
					select {
					case <-runCtx.Done():
						w.Logger.Info().Str("event_id", event.GetEventID()).
							Msg("skipping processing due to shutdown")
						observ.PromEventTotalProcessStatus.WithLabelValues("skipped", EventType).Inc()
						return
					default:

					}

					// Increment retry counter before retrying
					observ.PromEventRetryCount.WithLabelValues(EventType).Inc()

					err := w.processEvent(spanCtx, event)
					if err != nil {
						w.Logger.Error().Err(err).
							Str("event_id", event.GetEventID()).
							Msg("event processing failed permanently")

						span.RecordError(err)
						span.SetStatus(codes.Error, "event processing failed permanently")
						// Add to the number of failed processed events metrics
						observ.PromEventTotalProcessStatus.WithLabelValues("failed", EventType).Inc()
						observ.PromEventTotalProcessed.WithLabelValues().Inc()
						span.End()
						return
					}
				}

				w.Logger.Info().
					Str("event_id", event.GetEventID()).
					Msg("finished processing of the event")
				// Record the event processing duration
				processingDuration := time.Since(eventProcessingStart).Seconds()
				observ.PromEventProcessingDuration.WithLabelValues(EventType).Observe(processingDuration)

				// Add to the number of successful processed events metrics
				observ.PromEventTotalProcessStatus.WithLabelValues("success", EventType).Inc()
				observ.PromEventTotalProcessed.WithLabelValues().Inc()
				span.End()
			}(nEvent)

		case <-runCtx.Done():
			w.Logger.Info().Msg("worker run loop exiting due to context cancellation")
			return
		}
	}
}

/*
Shutdown function of the worker to shut it down gracefully
*/
func (w *Worker) Shutdown(ctx context.Context) error {
	w.Logger.Info().Msg("initiating worker shutdown")

	w.Cancel() // cancel the worker job

	// Create a channel to signal when WaitGroup is done
	done := make(chan struct{})

	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		w.Logger.Warn().Msg("worker graceful shutdown timed out")
		return ctx.Err()
	case <-done:
		w.Logger.Info().Msg("worker shutdown completed successfully")
		return nil
	}
}

/*
processEvent simulate processing of an event by doing digest calculation
*/
func (w *Worker) processEvent(ctx context.Context, event data.Event) error {
	ctx, span := otel.Tracer("Worker.ProcessEvent.Tracer").Start(ctx, "Worker.ProcessEvent.Span")
	defer span.End()
	span.SetAttributes(attribute.String("event.id", event.GetEventID()))

	startTime := time.Now()

	eMeta := event.GetMetadata()

	// Now serialize the metadata with the updated ThreadID
	jMeta, err := helpers.MarshalJson(ctx, eMeta)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to serialize the event metadata to json format")
		return err
	}

	// calculate the hash of the metadata
	hasher := md5.New()
	hasher.Write(jMeta)
	metaHashHex := hex.EncodeToString(hasher.Sum(nil))
	// calculate the length of the metadata
	metaLength := len(jMeta)

	// retrive the amount of time spent on calculating hash and length and goroutine id
	firstPhaseProcessTime := time.Since(startTime)

	// Get goroutine ID and update the event's ThreadID
	metaGoroutineId := helpers.GetGoroutineID(ctx)

	if logEvent, ok := event.(*data.EventLog); ok {
		logEvent.BaseEvent.ThreadID = int(metaGoroutineId)
	} else if metricEvent, ok := event.(*data.EventMetric); ok {
		metricEvent.BaseEvent.ThreadID = int(metaGoroutineId)
	}

	// simulate an additional processing time for the metadata
	randomTime := 0.05 + rand.Float32()*(0.2-0.05)
	time.Sleep(time.Duration(randomTime))

	metaProcessingTime := randomTime + float32(firstPhaseProcessTime.Seconds())

	// show the process finishing time
	metaProcessAt := time.Now()

	processResult := struct {
		Event          data.Event
		Md5            string
		Length         int
		ProcessingTime string
		ProcessedAt    time.Time
	}{
		Event:          event,
		Md5:            metaHashHex,
		Length:         metaLength,
		ProcessingTime: fmt.Sprintf("%.4f", metaProcessingTime),
		ProcessedAt:    metaProcessAt,
	}

	jResult, err := helpers.MarshalJson(ctx, processResult)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to serialize the event metadata to json format")
		return err
	}

	w.fileLock.Lock()
	defer w.fileLock.Unlock()

	file, err := os.OpenFile(CmdProcessedEventFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0660)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, fmt.Sprintf("failed to open the %s to persist event processing info", CmdProcessedEventFile))
		return err
	}
	defer file.Close()

	_, err = file.Write(jResult)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, fmt.Sprintf("failed persist the event processing information in %s", CmdProcessedEventFile))
		return err
	}

	return nil
}
