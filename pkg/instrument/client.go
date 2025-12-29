package instrument

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/ricoberger/echoserver/pkg/version"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	promexp "go.opentelemetry.io/otel/exporters/prometheus"
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

func New(ctx context.Context) (Client, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
		b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader|b3.B3SingleHeader)),
	))

	defaultResource, err := newReource(ctx)
	if err != nil {
		return nil, err
	}

	loggerProvider, err := newLoggerProvider(ctx, defaultResource)
	if err != nil {
		return nil, err
	}
	global.SetLoggerProvider(loggerProvider)

	meterProvider, err := newMeterProvider(ctx, defaultResource)
	if err != nil {
		return nil, err
	}
	otel.SetMeterProvider(meterProvider)

	tracerProvider, err := newTracerProvider(ctx, defaultResource)
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

func newReource(ctx context.Context) (*resource.Resource, error) {
	options := []resource.Option{
		resource.WithAttributes(attribute.Key("service.name").String("echoserver")),
		resource.WithAttributes(attribute.Key("service.version").String(version.Version)),
		resource.WithFromEnv(),
	}

	for detector := range strings.SplitSeq(os.Getenv("OTEL_RESOURCE_DETECTORS"), ",") {
		switch detector {
		case "container":
			options = append(options, resource.WithContainer())
		case "host":
			options = append(options, resource.WithHost())
		case "os":
			options = append(options, resource.WithOS())
		case "process":
			options = append(options, resource.WithProcess())
		case "sdk":
			options = append(options, resource.WithTelemetrySDK())
		}
	}

	return resource.New(ctx, options...)
}

func newLoggerProvider(ctx context.Context, defaultResource *resource.Resource) (*log.LoggerProvider, error) {
	switch os.Getenv("OTEL_LOGS_EXPORTER") {
	case "console":
		// exp, err := stdoutlog.New()
		// if err != nil {
		// 	return nil, err
		// }
		//
		// lp := log.NewLoggerProvider(
		// 	log.WithProcessor(log.NewBatchProcessor(exp)),
		// 	log.WithResource(defaultResource),
		// )
		// slog.SetDefault(otelslog.NewLogger("echoserver", otelslog.WithSource(true), otelslog.WithVersion(version.Version)))
		// return lp, nil

		// Instead of the stdout exporter we use a simple slog logger for
		// better readability.
		setupConsoleLogger()

		lp := log.NewLoggerProvider()
		lp.LoggerProvider = logNoop.NewLoggerProvider()
		return lp, nil
	case "otlp":
		exp, err := otlploggrpc.New(ctx, otlploggrpc.WithInsecure())
		if err != nil {
			return nil, err
		}

		lp := log.NewLoggerProvider(
			log.WithProcessor(log.NewBatchProcessor(exp)),
			log.WithResource(defaultResource),
		)
		slog.SetDefault(otelslog.NewLogger("echoserver", otelslog.WithSource(true), otelslog.WithVersion(version.Version)))
		return lp, nil
	default:
		lp := log.NewLoggerProvider()
		lp.LoggerProvider = logNoop.NewLoggerProvider()
		return lp, nil
	}
}

func newMeterProvider(ctx context.Context, defaultResource *resource.Resource) (*metric.MeterProvider, error) {
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
		exp, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithInsecure())
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

func newTracerProvider(ctx context.Context, defaultResource *resource.Resource) (*trace.TracerProvider, error) {
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
		exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
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
