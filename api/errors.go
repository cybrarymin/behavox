package api

import (
	"context"
	"fmt"
	"net/http"

	helpers "github.com/cybrarymin/behavox/internal"
)

// logError is the method we use to log the errors hapiens on the server side for the ApiServer.
func (api *ApiServer) logError(err error) {
	api.Logger.Error().Err(err).Send()
}

// errorResponse is the method we use to send a json formatted error to the client in case of any error
func (api *ApiServer) errorResponse(w http.ResponseWriter, r *http.Request, status int, message interface{}) {
	ctx := context.Background()
	e := helpers.Envelope{
		"error": message,
	}
	err := helpers.WriteJson(ctx, w, status, e, nil)

	if err != nil {
		api.logError(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// serverErrorResponse uses the two other methods to log the details of the error and send internal server error to the client
func (api *ApiServer) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	api.logError(err)
	message := "the server encountered an error to process the request"
	api.errorResponse(w, r, http.StatusInternalServerError, message)
}

// notFoundResponse method will be used to send notFound 404 status error json response to the client
func (api *ApiServer) notFoundResponse(w http.ResponseWriter, r *http.Request) {
	message := "the requested resource couldn't be found"
	api.errorResponse(w, r, http.StatusNotFound, message)
}

// badRequestResponse method will be used to send notFound 400 status error json response to the client
func (api *ApiServer) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	api.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

// methodNotAllowed method will be used to send notFound 404 status error json response to the client
func (api *ApiServer) methodNotAllowedResponse(w http.ResponseWriter, r *http.Request) {
	message := fmt.Sprintf("the %s method is not supported for this resource", r.Method)
	api.errorResponse(w, r, http.StatusMethodNotAllowed, message)
}

// failedValidationResponse method will be used to send 422 status error json response to the client for invalid input
func (api *ApiServer) failedValidationResponse(w http.ResponseWriter, r *http.Request, errors map[string]string) {
	api.errorResponse(w, r, http.StatusUnprocessableEntity, errors)
}

// func (api *ApiServer) editConflictResponse(w http.ResponseWriter, r *http.Request) {
// 	message := "unable to update the record due to an edit conflict, please try again"
// 	api.errorResponse(w, r, http.StatusConflict, message)
// }

func (api *ApiServer) rateLimitExceedResponse(w http.ResponseWriter, r *http.Request) {
	message := "request rate limit reached, please try again later"
	api.errorResponse(w, r, http.StatusTooManyRequests, message)
}

func (api *ApiServer) invalidAuthenticationCredResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer Jwt")
	message := "invalid authentication creds or token"
	api.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (api *ApiServer) invalidJWTTokenSignatureResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer Jwt")
	message := "invalid jwt token signature."
	api.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (api *ApiServer) authenticationRequiredResposne(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer Jwt")
	message := "authentication required"
	api.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (api *ApiServer) unauthorizedAccessInactiveUserResponse(w http.ResponseWriter, r *http.Request) {
	message := "user must be activated to access this resource"
	api.errorResponse(w, r, http.StatusForbidden, message)
}

func (api *ApiServer) notPermittedResponse(w http.ResponseWriter, r *http.Request) {
	message := "your user account doesn't have the necessary permissions to access this resource"
	api.errorResponse(w, r, http.StatusForbidden, message)
}
