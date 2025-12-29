package instrument

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

func setupConsoleLogger() *slog.Logger {
	var handler slog.Handler

	var level slog.Level
	level.UnmarshalText([]byte(os.Getenv("LOG_LEVEL")))

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

	handler = &customHandler{handler}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

type customHandler struct {
	slog.Handler
}

func (h *customHandler) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	r.Add("trace_id", slog.StringValue(span.SpanContext().TraceID().String()))
	r.Add("trace_flags", slog.StringValue(span.SpanContext().TraceFlags().String()))
	r.Add("span_id", slog.StringValue(span.SpanContext().SpanID().String()))

	return h.Handler.Handle(ctx, r)
}

func (c *customHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return c.clone()
}

func (c *customHandler) clone() *customHandler {
	clone := *c
	return &clone
}
