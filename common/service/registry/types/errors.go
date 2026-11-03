package types

import (
	"errors"
	"fmt"
	"net/http"
)

const (
	// NotFoundCode is code for the error of no object found
	NotFoundCode = "NOT_FOUND"
	// UnAuthorizedCode ...
	UnAuthorizedCode = "UNAUTHORIZED"
	// ForbiddenCode ...
	ForbiddenCode = "FORBIDDEN"
	// RateLimitCode
	RateLimitCode = "TOO_MANY_REQUEST"
	// GeneralCode ...
	GeneralCode = "UNKNOWN"
)

// HTTPError preserves status codes so missing images can be distinguished from
// authentication, transport, and registry failures without parsing messages.
type HTTPError struct {
	StatusCode int
	Code       string
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s: http status code: %d, body: %s", e.Code, e.StatusCode, e.Body)
}

func IsNotFound(err error) bool {
	var responseErr *HTTPError
	return errors.As(err, &responseErr) && responseErr.StatusCode == http.StatusNotFound
}
