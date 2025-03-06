package logger

import (
	"context"
	"log/slog"
	"os"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel/trace"
)

type ctxSlogFieldsKey int

const slogFields ctxSlogFieldsKey = 0

var (
	logCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "echoserver",
		Name:      "logs_total",
		Help:      "Number of logs, partitioned by log level.",
	}, []string{"level"})
)

// Config is the configuration for the log package. Within the configuration it
// is possible to set the log level and log format for our logger.
type Config struct {
	Format string     `env:"FORMAT" enum:"console,json" default:"console" help:"Set the output format of the logs. Must be \"console\" or \"json\"."`
	Level  slog.Level `env:"LEVEL" enum:"DEBUG,INFO,WARN,ERROR" default:"INFO" help:"Set the log level. Must be \"DEBUG\", \"INFO\", \"WARN\" or \"ERROR\"."`
}

func New(config Config) *slog.Logger {
	var handler slog.Handler

	if config.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     config.Level,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     config.Level,
		})
	}

	handler = &CustomHandler{handler}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

// CustomHandler is a custom handler for our logger, which adds the request Id
// and trace Id to the log record, if they exists in the provided context.
type CustomHandler struct {
	slog.Handler
}

func (h *CustomHandler) Handle(ctx context.Context, r slog.Record) error {
	logCount.WithLabelValues(r.Level.String()).Inc()

	if requestId := middleware.GetReqID(ctx); requestId != "" {
		r.Add("requestID", slog.StringValue(requestId))
	}

	if span := trace.SpanFromContext(ctx); span.SpanContext().HasTraceID() {
		r.Add("traceID", slog.StringValue(span.SpanContext().TraceID().String()))
	}

	if attrs, ok := ctx.Value(slogFields).([]slog.Attr); ok {
		for _, v := range attrs {
			r.AddAttrs(v)
		}
	}

	return h.Handler.Handle(ctx, r)
}

func (c *CustomHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return c.clone()
}

func (c *CustomHandler) clone() *CustomHandler {
	clone := *c
	return &clone
}

func AppendCtx(parent context.Context, attrs ...slog.Attr) context.Context {
	if parent == nil {
		parent = context.Background()
	}

	if v, ok := parent.Value(slogFields).([]slog.Attr); ok {
		v = append(v, attrs...)
		return context.WithValue(parent, slogFields, v)
	}

	v := []slog.Attr{}
	v = append(v, attrs...)
	return context.WithValue(parent, slogFields, v)
}
