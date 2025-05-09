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
	"go.opentelemetry.io/otel/codes"
)

var (
	CmdProcessedEventFile string
)

type Worker struct {
	wg         sync.WaitGroup
	Logger     *zerolog.Logger
	EventQueue *data.EventQueue
	Ctx        context.Context
	Cancel     context.CancelFunc
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

	runCtx := w.Ctx

	w.wg.Add(1)
	defer w.wg.Done()
	for {
		select {
		case nEvent := <-w.EventQueue.Events:
			spanCtx, span := otel.Tracer("Worker.Tracer").Start(ctx, "Worker.Span")
			var EventType string
			switch nEvent.(type) {
			case *data.EventLog:
				EventType = "log"
			case *data.EventMetric:
				EventType = "metric"
			}

			// Measure queue wait time (time from enqueue to processing)
			var queueWaitTime float64
			if baseEvent, ok := nEvent.(*data.EventLog); ok && !baseEvent.BaseEvent.EnqueueTime.IsZero() {
				queueWaitTime = time.Since(baseEvent.BaseEvent.EnqueueTime).Seconds()
				observ.PromEventQueueWaitTime.WithLabelValues(EventType).Observe(queueWaitTime)
			} else if baseEvent, ok := nEvent.(*data.EventMetric); ok && !baseEvent.BaseEvent.EnqueueTime.IsZero() {
				queueWaitTime = time.Since(baseEvent.BaseEvent.EnqueueTime).Seconds()
				observ.PromEventQueueWaitTime.WithLabelValues(EventType).Observe(queueWaitTime)
			}

			// Capture the start time for event processing duration
			eventProcessingStart := time.Now()

			err := processEvent(spanCtx, nEvent)
			if err != nil {
				w.Logger.Error().Err(err).
					Str("event_id", nEvent.GetEventID()).
					Msg("event processing failed")

				time.Sleep(2 * time.Second) // wait for two second and reprocess the event
				// Check if context is cancelled before retry
				select {
				case <-runCtx.Done():
					w.Logger.Info().Str("event_id", nEvent.GetEventID()).
						Msg("skipping processing due to shutdown")
					observ.PromEventTotalProcessStatus.WithLabelValues("skipped", EventType).Inc()
					return
				default:

				}

				// Increment retry counter before retrying
				observ.PromEventRetryCount.WithLabelValues(EventType).Inc()

				err := processEvent(spanCtx, nEvent)
				if err != nil {
					w.Logger.Error().Err(err).
						Str("event_id", nEvent.GetEventID()).
						Msg("event processing failed permanently")

					span.RecordError(err)
					span.SetStatus(codes.Error, "event processing failed permanently")
					// Add to the number of failed processed events metrics
					observ.PromEventTotalProcessStatus.WithLabelValues("failed", EventType).Inc()
					observ.PromEventTotalProcessed.WithLabelValues().Inc()
					span.End()
					continue
				}
			}

			// Record the event processing duration
			processingDuration := time.Since(eventProcessingStart).Seconds()
			observ.PromEventProcessingDuration.WithLabelValues(EventType).Observe(processingDuration)

			// Add to the number of successful processed events metrics
			observ.PromEventTotalProcessStatus.WithLabelValues("success", EventType).Inc()
			observ.PromEventTotalProcessed.WithLabelValues().Inc()
			span.End()

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
func processEvent(ctx context.Context, event data.Event) error {
	ctx, span := otel.Tracer("Worker.ProcessEvent.Tracer").Start(ctx, "Worker.ProcessEvent.Span")
	defer span.End()

	startTime := time.Now()

	eMeta := event.GetMetadata()
	// serialize the metadata do json format
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
	// fetch the thread id of the event
	metaGoroutineId := helpers.GetGoroutineID(ctx)

	// retrive the amount of time spent on calculating hash and length and goroutine id
	firstPhaseProcessTime := time.Since(startTime)

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
		GoRoutineID    uint64
		ProcessingTime string
		ProcessedAt    time.Time
	}{
		Event:          event,
		Md5:            metaHashHex,
		Length:         metaLength,
		GoRoutineID:    metaGoroutineId,
		ProcessingTime: fmt.Sprintf("%.4f", metaProcessingTime),
		ProcessedAt:    metaProcessAt,
	}

	file, err := os.OpenFile(CmdProcessedEventFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0660)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, fmt.Sprintf("failed to open the %s to persist event processing info", CmdProcessedEventFile))
		return err
	}
	defer file.Close()

	jResult, err := helpers.MarshalJson(ctx, processResult)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to serialize the event metadata to json format")
		return err
	}
	_, err = file.Write(jResult)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, fmt.Sprintf("failed persist the event processing information in %s", CmdProcessedEventFile))
		return err
	}

	return nil
}
