package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (api *ApiServer) routes() http.Handler {
	router := httprouter.New()

	// handle error responses for both notFoundResponses and InvalidMethods
	router.NotFound = api.promHandler(http.HandlerFunc(api.notFoundResponse))
	router.MethodNotAllowed = api.promHandler(api.methodNotAllowedResponse)

	// handle the event
	router.HandlerFunc(http.MethodPost, "/v1/events", api.promHandler(api.createEventHandler))
	router.HandlerFunc(http.MethodGet, "/v1/events/stats", api.promHandler(api.GetEventStatsHandler))

	// Prometheus Handler
	router.Handler(http.MethodGet, "/metrics", promhttp.Handler())

	// Otel http instrumentation
	return api.panicRecovery(api.setContextHandler(api.otelHandler(api.rateLimit(router))))
}
