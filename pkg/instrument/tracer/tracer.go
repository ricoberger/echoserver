package tracer

import (
	"context"
	"log/slog"
	"time"

	"github.com/ricoberger/echoserver/pkg/version"

	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

// Config is the configuration for our tracer. Via the configuration we can
// enable / disable the tracing. If the tracing is enabled we need the service
// name and address.
type Config struct {
	Enabled bool   `env:"ENABLED" enum:"true,false" default:"false" help:"Enable tracing."`
	Service string `env:"SERVICE" default:"echoserver" help:"The name of the service which should be used for tracing."`
	Address string `env:"ADDRESS" default:"localhost:4317" help:"The address of the tracing provider instance."`
}

// Client is the interface for our tracer. It contains the underlying tracer
// provider and a Shutdown method to perform a clean shutdown.
type Client interface {
	Shutdown()
}

type client struct {
	tracerProvider *tracesdk.TracerProvider
}

// Shutdown is used to gracefully shutdown the tracer provider, created during
// the setup. The gracefull shutdown can take at the maximum 3 seconds.
func (c *client) Shutdown() {
	if c.tracerProvider == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := c.tracerProvider.Shutdown(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Graceful shutdown of the tracer provider failed.", slog.Any("error", err))
	}
}

// New is used to create a new tracer. For that we are creating a new
// TracerProvider and register it as the global so any imported instrumentation
// will default to using it. If tracing is disabled the setup function returns
// a client without a TracerProvider.
//
// During the shutdown process of echoserver the "Shutdown" method of the
// returned client must be called.
//
// To test the tracer we can start a local Jaeger instance with the following
// commands (the UI will be available at localhost:16686):
//
//	docker run --rm --name jaeger -d -p 16686:16686 -p 4317:4317 jaegertracing/all-in-one:1.54
//	docker stop jaeger
func New(config Config) (Client, error) {
	if !config.Enabled {
		return &client{}, nil
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
		b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader|b3.B3SingleHeader)),
	))

	tp, err := newProvider(config)
	if err != nil {
		return nil, err
	}

	otel.SetTracerProvider(tp)

	return &client{
		tracerProvider: tp,
	}, nil
}

// newProvider returns an OpenTelemetry TracerProvider configured to use the
// OTLP gRPC exporter that will send spans to the provided url. The returned
// TracerProvider will also use a Resource configured with all the information
// about the application.
func newProvider(config Config) (*tracesdk.TracerProvider, error) {
	exp, err := otlptracegrpc.New(
		context.Background(),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(config.Address),
	)
	if err != nil {
		return nil, err
	}

	defaultResource, err := resource.New(
		context.Background(),
		resource.WithAttributes(attribute.Key("service.name").String(config.Service)),
		resource.WithAttributes(attribute.Key("service.version").String(version.Version)),
		resource.WithContainer(),
		resource.WithContainerID(),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcessExecutableName(),
		resource.WithProcessExecutablePath(),
		resource.WithProcessOwner(),
		resource.WithProcessPID(),
		resource.WithProcessRuntimeDescription(),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, err
	}

	return tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(defaultResource),
	), nil
}
