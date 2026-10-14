package provider

import (
	"errors"
	"fmt"
)

// HTTPError retains only the status. Upstream bodies can contain credentials or
// private data and must never be exposed through administrative diagnostics.
type HTTPError struct{ StatusCode int }

func (e *HTTPError) Error() string { return fmt.Sprintf("provider returned status %d", e.StatusCode) }

var ErrEmptyContent = errors.New("provider returned empty content")
