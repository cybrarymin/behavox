package api

import (
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	apiObserv "github.com/cybrarymin/behavox/api/observability"
	"github.com/felixge/httpsnoop"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

/*
setContextHandler sets the required key, values on the http.request context
*/
func (api *ApiServer) setContextHandler(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r = api.setReqIDContext(r)
		next.ServeHTTP(w, r)
	}
}

/*
panicRecovery handler is gonna be used to avoid server sending empty reply as a response to the client when a panic happens.
The server will recover the panic and sends http status code 500 with internal error to the client and logs the panic with stack.
*/
func (api *ApiServer) panicRecovery(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if panicErr := recover(); panicErr != nil {
				// Setting this header will trigger the HTTP server to close the connection after Panic happended
				w.Header().Set("Connection", "close")
				api.serverErrorResponse(w, r, fmt.Errorf("%s, %s", panicErr, debug.Stack()))
			}
		}()
		next.ServeHTTP(w, r)
	}
}

/*
otelHandler is gonna instrument the otel http handler
*/
func (api *ApiServer) otelHandler(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		/*
			adding the request id to the context to be visible inside the span so each request can be tracable
		*/
		newNext := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := api.getReqIDContext(r)
			span := trace.SpanFromContext(r.Context())
			if reqID != "" {
				span.SetAttributes(attribute.String("http.request.id", fmt.Sprintf("%v", reqID)))
			}
			next.ServeHTTP(w, r)
		})

		nHander := otelhttp.NewHandler(newNext, "otel.instrumented.handler")
		nHander.ServeHTTP(w, r)

	}
}

/*
promHandler is gonna expose and calculate the prometheus metrics values on each api path.
*/
func (api *ApiServer) promHandler(next http.HandlerFunc, path string) http.HandlerFunc {
	apiObserv.PromApplicationVersion.WithLabelValues(Version).Set(1)
	return func(w http.ResponseWriter, r *http.Request) {
		apiObserv.PromHttpTotalRequests.WithLabelValues().Inc()
		pTimer := prometheus.NewTimer(apiObserv.PromHttpDuration.WithLabelValues(path))
		defer pTimer.ObserveDuration()
		snoopMetrics := httpsnoop.CaptureMetrics(next, w, r)
		apiObserv.PromHttpTotalResponse.WithLabelValues().Inc()
		apiObserv.PromHttpResponseStatus.WithLabelValues(path, strconv.Itoa(snoopMetrics.Code)).Inc()
	}
}

/*
rateLimited is api rateLimitter middleware which blocks requests processing from same client ip more than specified threshold
*/
type ClientRateLimiter struct {
	Limit          *rate.Limiter
	LastAccessTime *time.Timer
}

func (api *ApiServer) rateLimit(next http.Handler) http.Handler {
	if api.Cfg.RateLimit.Enabled {
		// Global rate limiter
		busrtSize := api.Cfg.RateLimit.GlobalRateLimit + api.Cfg.RateLimit.GlobalRateLimit/10
		nRL := rate.NewLimiter(rate.Limit(api.Cfg.RateLimit.GlobalRateLimit), int(busrtSize))

		// Per IP or Per Client rate limiter
		pcbusrtSize := api.Cfg.RateLimit.perClientRateLimit + api.Cfg.RateLimit.perClientRateLimit/10
		pcnRL := make(map[string]ClientRateLimiter)

		expirationTime := 30 * time.Second

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !nRL.Allow() { // In this code, whenever we call the Allow() method on the rate limiter exactly one token will be consumed from the bucket. And if there is no token in the bucket left Allow() will return false
				api.rateLimitExceedResponse(w, r)
				return
			}

			// Getting client address from the http remoteAddr heder
			clientAddr, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				api.serverErrorResponse(w, r, err)
				return
			}

			api.mu.RLock()
			_, found := pcnRL[clientAddr]
			api.mu.RUnlock()

			// Check to see if the client address already exists inside the memory or not.
			// If not adding the client ip address to the memory and updating the last access time of the client
			if !found {
				api.mu.Lock()
				pcnRL[clientAddr] = ClientRateLimiter{
					rate.NewLimiter(rate.Limit(api.Cfg.RateLimit.perClientRateLimit), int(pcbusrtSize)),
					time.NewTimer(expirationTime),
				}
				api.mu.Unlock()

				go func() {
					<-pcnRL[clientAddr].LastAccessTime.C
					api.mu.RLock()
					delete(pcnRL, clientAddr)
					api.mu.RUnlock()
				}()

			} else {
				api.mu.Lock()
				api.Logger.Debug().Msgf("renewing client %v expiry of rate limiting context", clientAddr)
				pcnRL[clientAddr].LastAccessTime.Reset(expirationTime)
				api.mu.Unlock()
			}

			api.mu.RLock()
			allow := pcnRL[clientAddr].Limit.Allow()
			api.mu.RUnlock()

			if !allow {
				api.rateLimitExceedResponse(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	} else {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}
