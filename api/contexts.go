package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const RequestContextKey = contextKey("request_id")

/*
setReqIDContext is used to generate a unique request id and set it on http.request context.
*/
func (api *ApiServer) setReqIDContext(r *http.Request) *http.Request {
	reqId := uuid.New()
	nCtx := context.WithValue(r.Context(), RequestContextKey, reqId.String())
	r = r.WithContext(nCtx)
	return r
}

/*
getReqIDContext is used to get the unique request id from http.request context.
*/
func (api *ApiServer) getReqIDContext(r *http.Request) string {
	reqID := r.Context().Value(RequestContextKey)
	return reqID.(string)
}
