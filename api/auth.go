package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	helpers "github.com/cybrarymin/behavox/internal"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var (
	CmdJwtKey       string
	CmdApiAdmin     string
	CmdApiAdminPass string
)

type customClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

/*
This function is used comletely to implement jwt.claimsValidator.
When we define this function for our customClaim then jwt.Validator will validate our custom claim after the registered claim based on this function
*/
func (c *customClaims) Validate() error {
	if ok := helpers.EmailRX.MatchString(c.Email); !ok {
		return errors.New("invalid email claim on jwt token")
	}
	return nil
}

/*
Authenticating user using basic authentication method. If user is valid it's gonna issue a JWT Token to the user
*/
func (api *ApiServer) createJWTTokenHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("createJWTToken.handler.tracer").Start(r.Context(), "createJWTToken.handler.span")
	defer span.End()

	ok, nUser := api.BasicAuth(w, r)
	if !ok {
		return
	}
	claims := customClaims{
		Email: nUser + "@behavox.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "behavox.example.com",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24 * 3)),
			Subject:   nUser,
			Audience:  []string{"behavox.example.com"},
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
	}
	span.SetAttributes(attribute.String("claims.user", claims.Email))
	span.SetAttributes(attribute.String("claims.issuer", claims.Issuer))
	span.SetAttributes(attribute.String("claims.subject", claims.Subject))
	span.SetAttributes(attribute.StringSlice("claims.audience", claims.Audience))
	span.SetAttributes(attribute.String("claims.id", claims.ID))

	jToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims, func(t *jwt.Token) {})

	signedToken, err := jToken.SignedString([]byte(CmdJwtKey))
	if err != nil {
		api.serverErrorResponse(w, r, err)
		return
	}
	err = helpers.WriteJson(ctx, w, http.StatusOK, helpers.Envelope{"result": map[string]string{"token": signedToken}}, nil)
	if err != nil {
		api.serverErrorResponse(w, r, err)
		return
	}
}

/*
Authenticates the user using basic authentication method.
in case of successfull authentication it returns ok plus userinfo
*/
func (api *ApiServer) BasicAuth(w http.ResponseWriter, r *http.Request) (bool, string) {
	_, span := otel.Tracer("basicAuth.handler.Tracer").Start(r.Context(), "basicAuth.handler.Span")
	defer span.End()

	user, pass, ok := r.BasicAuth()
	if !ok {
		span.SetStatus(codes.Error, "failed authentication")
		api.authenticationRequiredResposne(w, r)
		return false, ""
	}
	nVal := helpers.NewValidator()
	nVal.Check(user != "", "name", "must be provided")
	nVal.Check(len(user) <= 500, "name", "must not be more than 500 bytes long")
	nVal.Check(pass != "", "password", "must be provided")
	nVal.Check(len(pass) >= 8, "password", "must be at least 8 bytes long")
	nVal.Check(len(pass) <= 72, "password", "must not be more than 72 bytes long")

	if !nVal.Valid() {
		for k, v := range nVal.Errors {
			span.RecordError(fmt.Errorf("%s : %s", k, v))
		}
		span.SetStatus(codes.Error, "failed authentication")
		api.invalidAuthenticationCredResponse(w, r)
		return false, ""
	}

	if user != CmdApiAdmin || pass != CmdApiAdminPass {
		span.SetStatus(codes.Error, "failed authentication due to invalid username or password")
		api.invalidAuthenticationCredResponse(w, r)
		return false, ""
	}

	return true, user
}
