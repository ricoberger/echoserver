package logger

import (
	"context"
	"log/slog"
	"testing"

	"github.com/ricoberger/echoserver/pkg/instrument/tracer"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

func TestNew(t *testing.T) {
	t.Run("should succeed with valid config", func(t *testing.T) {
		logger := New(Config{Level: slog.LevelDebug, Format: "json"})
		require.NotNil(t, logger)
	})

	t.Run("should succeed with invalid config", func(t *testing.T) {
		logger := New(Config{Level: slog.LevelDebug, Format: "fmt"})
		require.NotNil(t, logger)
	})
}

func TestHandle(t *testing.T) {
	logger := New(Config{Level: slog.LevelDebug, Format: "json"})
	tracerClient, _ := tracer.New(tracer.Config{Enabled: true, Service: "test", Address: "localhost:4317"})
	defer tracerClient.Shutdown()

	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.RequestIDKey, "requestID")
	ctx, span := otel.Tracer("test").Start(ctx, "test")
	defer span.End()

	require.NotPanics(t, func() {
		logger.DebugContext(ctx, "test")
	})
}
