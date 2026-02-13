package instrument

import (
	"context"
	"log/slog"
	"os"

	"github.com/ricoberger/echoserver/pkg/httpserver/middleware/requestid"

	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
)

func setupConsoleLogger(defaultResource *resource.Resource) *slog.Logger {
	var handler slog.Handler

	var level slog.Level
	if os.Getenv("LOG_LEVEL") != "" {
		level.UnmarshalText([]byte(os.Getenv("LOG_LEVEL")))
	} else {
		level = slog.LevelInfo
	}

	if os.Getenv("LOG_FORMAT") == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     level,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     level,
		})
	}

	// If the LOG_RESOURCE_ATTRIBUTES environment variable is set to true,
	// we add the resource attributes to each log record.
	if os.Getenv("LOG_RESOURCE_ATTRIBUTES") == "true" {
		handler = &CustomHandler{handler, defaultResource}
	} else {
		handler = &CustomHandler{handler, nil}
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

type CustomHandler struct {
	slog.Handler
	*resource.Resource
}

func (h *CustomHandler) Handle(ctx context.Context, r slog.Record) error {
	if requestId := requestid.Get(ctx); requestId != "" {
		r.Add("http.request.header.x-request-id", slog.StringValue(requestId))
	}

	span := trace.SpanContextFromContext(ctx)
	if span.HasTraceID() && span.HasSpanID() {
		r.Add("trace_id", slog.StringValue(span.TraceID().String()))
		r.Add("trace_flags", slog.StringValue(span.TraceFlags().String()))
		r.Add("span_id", slog.StringValue(span.SpanID().String()))
	}

	if h.Resource != nil {
		for _, attr := range h.Attributes() {
			r.Add(string(attr.Key), slog.StringValue(attr.Value.AsString()))
		}
	}

	return h.Handler.Handle(ctx, r)
}
