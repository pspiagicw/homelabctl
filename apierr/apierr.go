package apierr

import (
	"errors"
)

// Singleton errors for HTTP status mapping
var (
	ErrNotFound    = errors.New("not found")
	ErrConflict    = errors.New("conflict")
	ErrUnavailable = errors.New("unavailable")
	ErrUpstream    = errors.New("upstream")
)
