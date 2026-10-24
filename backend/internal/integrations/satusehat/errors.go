package satusehat

import (
	"fmt"
	"time"
)

type Category string

const (
	CategoryAuth        Category = "auth_rejected"
	CategoryForbidden   Category = "forbidden"
	CategoryBadRequest  Category = "bad_request"
	CategoryRateLimited Category = "rate_limited"
	CategoryUpstream    Category = "upstream_unavailable"
	CategoryTimeout     Category = "timeout"
	CategoryNetwork     Category = "network"
	CategoryInvalid     Category = "invalid_response"
	CategoryCanceled    Category = "canceled"
)

// Error carries only an operation, a category and an HTTP status. It never includes
// request bodies, headers, tokens or credentials, so it is safe to print and log.
type Error struct {
	Op       string
	Category Category
	Status   int
	Attempts int
	err      error
}

func (e *Error) Error() string {
	s := "satusehat " + e.Op + ": " + string(e.Category)
	if e.Status != 0 {
		s += fmt.Sprintf(" (HTTP %d)", e.Status)
	}
	if e.Attempts > 1 {
		s += fmt.Sprintf(" after %d attempts", e.Attempts)
	}
	return s
}

// Unwrap exposes context errors for errors.Is; other causes stay hidden because
// net/http errors embed request URLs.
func (e *Error) Unwrap() error { return e.err }

func statusCategory(status int) Category {
	switch {
	case status == 401:
		return CategoryAuth
	case status == 403:
		return CategoryForbidden
	case status == 429:
		return CategoryRateLimited
	case status >= 500:
		return CategoryUpstream
	default:
		return CategoryBadRequest
	}
}

// parseRetryAfter supports delta-seconds and HTTP-date forms. ok is false when absent or invalid.
func parseRetryAfter(v string, now time.Time) (time.Duration, bool) {
	if v == "" {
		return 0, false
	}
	var seconds int
	if _, e := fmt.Sscanf(v, "%d", &seconds); e == nil && fmt.Sprint(seconds) == v {
		if seconds < 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}
	if t, e := time.Parse(time.RFC1123, v); e == nil {
		if d := t.Sub(now); d > 0 {
			return d, true
		}
		return 0, true
	}
	return 0, false
}
