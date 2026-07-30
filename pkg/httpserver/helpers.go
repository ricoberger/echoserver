package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// parseRequiredIntParam parses a required integer query parameter. It returns
// an error when the parameter is missing or can not be parsed.
func parseRequiredIntParam(r *http.Request, name string) (int, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return 0, fmt.Errorf("%s parameter is missing", name)
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("failed to parse '%s' parameter: %w", name, err)
	}

	return parsed, nil
}

// parseRequiredDurationParam parses a required duration query parameter. It
// returns an error when the parameter is missing or can not be parsed.
func parseRequiredDurationParam(r *http.Request, name string) (time.Duration, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return 0, fmt.Errorf("%s parameter is missing", name)
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("failed to parse '%s' parameter: %w", name, err)
	}

	return parsed, nil
}

// handleError logs the given error, records it on the span and writes an error
// response with the given HTTP status code.
func handleError(ctx context.Context, w http.ResponseWriter, span trace.Span, status int, message string, err error) {
	slog.ErrorContext(ctx, message, slog.Any("error", err))
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())

	http.Error(w, err.Error(), status)
}
