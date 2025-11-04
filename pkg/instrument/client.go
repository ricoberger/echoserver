package instrument

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ricoberger/echoserver/pkg/version"

	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	promexp "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/log/global"
	logNoop "go.opentelemetry.io/otel/log/noop"
	metricNoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	traceNoop "go.opentelemetry.io/otel/trace/noop"
)

type Client interface {
	Shutdown()
}

type client struct {
	loggerProvider *log.LoggerProvider
	meterProvider  *metric.MeterProvider
	tracerProvider *trace.TracerProvider
}

func (c *client) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := c.loggerProvider.Shutdown(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Graceful shutdown of the logger provider failed.", slog.Any("error", err))
	}

	err = c.tracerProvider.Shutdown(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Graceful shutdown of the tracer provider failed.", slog.Any("error", err))
	}
}

func New(serviceName string) (Client, error) {
	if serviceName == "" {
		return nil, fmt.Errorf("service name must not be empty")
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
		b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader|b3.B3SingleHeader)),
	))

	defaultResource, err := resource.New(
		context.Background(),
		resource.WithAttributes(attribute.Key("service.name").String(serviceName)),
		resource.WithAttributes(attribute.Key("service.version").String(version.Version)),
		// resource.WithContainer(),
		// resource.WithContainerID(),
		// resource.WithHost(),
		// resource.WithOS(),
		// resource.WithProcessExecutableName(),
		// resource.WithProcessExecutablePath(),
		// resource.WithProcessOwner(),
		// resource.WithProcessPID(),
		// resource.WithProcessRuntimeDescription(),
		// resource.WithProcessRuntimeName(),
		// resource.WithProcessRuntimeVersion(),
		// resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, err
	}

	loggerProvider, err := newLoggerProvider(defaultResource)
	if err != nil {
		return nil, err
	}
	global.SetLoggerProvider(loggerProvider)

	meterProvider, err := newMeterProvider(defaultResource)
	if err != nil {
		return nil, err
	}
	otel.SetMeterProvider(meterProvider)

	tracerProvider, err := newTracerProvider(defaultResource)
	if err != nil {
		return nil, err
	}
	otel.SetTracerProvider(tracerProvider)

	return &client{
		loggerProvider: loggerProvider,
		meterProvider:  meterProvider,
		tracerProvider: tracerProvider,
	}, nil
}

func newLoggerProvider(defaultResource *resource.Resource) (*log.LoggerProvider, error) {
	switch os.Getenv("OTEL_LOGS_EXPORTER") {
	case "console":
		exp, err := stdoutlog.New()
		if err != nil {
			return nil, err
		}

		return log.NewLoggerProvider(
			log.WithProcessor(log.NewBatchProcessor(exp)),
			log.WithResource(defaultResource),
		), nil
	case "otlp":
		exp, err := otlploggrpc.New(context.Background())
		if err != nil {
			return nil, err
		}

		return log.NewLoggerProvider(
			log.WithProcessor(log.NewBatchProcessor(exp)),
			log.WithResource(defaultResource),
		), nil
	default:
		tp := log.NewLoggerProvider()
		tp.LoggerProvider = logNoop.NewLoggerProvider()
		return tp, nil
	}
}

func newMeterProvider(defaultResource *resource.Resource) (*metric.MeterProvider, error) {
	switch os.Getenv("OTEL_METRICS_EXPORTER") {
	case "console":
		exp, err := stdoutmetric.New()
		if err != nil {
			return nil, err
		}

		return metric.NewMeterProvider(
			metric.WithReader(
				metric.NewPeriodicReader(
					exp,
					metric.WithInterval(15*time.Second),
				),
			),
			metric.WithResource(defaultResource),
		), nil
	case "otlp":
		exp, err := otlpmetricgrpc.New(context.Background())
		if err != nil {
			return nil, err
		}

		return metric.NewMeterProvider(
			metric.WithReader(
				metric.NewPeriodicReader(
					exp,
					metric.WithInterval(15*time.Second),
				),
			),
			metric.WithResource(defaultResource),
		), nil
	case "prometheus":
		exp, err := promexp.New(
			promexp.WithNamespace("echoserver"),
			promexp.WithoutScopeInfo(),
		)
		if err != nil {
			return nil, err
		}

		return metric.NewMeterProvider(
			metric.WithReader(exp),
			metric.WithResource(defaultResource),
		), nil
	default:
		mp := metric.NewMeterProvider()
		mp.MeterProvider = metricNoop.NewMeterProvider()
		return mp, nil
	}
}

func newTracerProvider(defaultResource *resource.Resource) (*trace.TracerProvider, error) {
	switch os.Getenv("OTEL_TRACES_EXPORTER") {
	case "console":
		exp, err := stdouttrace.New()
		if err != nil {
			return nil, err
		}

		return trace.NewTracerProvider(
			trace.WithBatcher(exp),
			trace.WithResource(defaultResource),
		), nil
	case "otlp":
		exp, err := otlptracegrpc.New(
			context.Background(),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, err
		}

		return trace.NewTracerProvider(
			trace.WithBatcher(exp),
			trace.WithResource(defaultResource),
		), nil
	default:
		tp := trace.NewTracerProvider()
		tp.TracerProvider = traceNoop.NewTracerProvider()
		return tp, nil
	}
}
