package instrument

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/ricoberger/echoserver/pkg/version"

	"github.com/felixge/httpsnoop"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type ctxKeyRequestInfo int

const RequestInfoKey ctxKeyRequestInfo = 0

type RequestInfo struct {
	Metrics    *httpsnoop.Metrics
	TraceId    oteltrace.TraceID
	SpanId     oteltrace.SpanID
	TraceFlags oteltrace.TraceFlags
	TraceState oteltrace.TraceState
}

var (
	tracer = otel.Tracer("instrument")
	logger = otelslog.NewLogger("instrument", otelslog.WithSource(true), otelslog.WithVersion(version.Version))
	meter  = otel.Meter("instrument")

	reqCount    metric.Int64Counter
	reqDuration metric.Float64Histogram
	respSize    metric.Int64Histogram
)

func init() {
	reqCount, _ = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Number of HTTP requests processed, partitioned by status code, method and path."),
	)
	reqDuration, _ = meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("Latency of HTTP requests processed, partitioned by status code, method and path."),
	)
	respSize, _ = meter.Int64Histogram(
		"http_response_size_bytes",
		metric.WithDescription("Size of HTTP responses, partitioned by status code, method and path."),
	)
}

func Handler() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var requestInfo = &RequestInfo{}
			r = r.WithContext(context.WithValue(r.Context(), RequestInfoKey, requestInfo))

			handler := handleTraces(requestInfo)(next)

			requestInfo.Metrics = &httpsnoop.Metrics{
				Code: http.StatusOK,
			}
			requestInfo.Metrics.CaptureMetrics(w, func(ww http.ResponseWriter) {
				handler.ServeHTTP(ww, r)
			})
			handleMetricsAndLogs(r, requestInfo)
		})
	}
}

func handleTraces(requestInfo *RequestInfo) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			ctx, span := tracer.Start(ctx, "http.request", oteltrace.WithSpanKind(oteltrace.SpanKindServer))
			defer span.End()

			requestInfo.TraceId = span.SpanContext().TraceID()
			requestInfo.SpanId = span.SpanContext().SpanID()
			requestInfo.TraceFlags = span.SpanContext().TraceFlags()
			requestInfo.TraceState = span.SpanContext().TraceState()

			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}

			defer func() {
				// In go-chi/chi, full route pattern could only be extracted
				// once the request is executed
				// See: https://github.com/go-chi/chi/issues/150#issuecomment-278850733
				routeStr := strings.Join(chi.RouteContext(r.Context()).RoutePatterns, "")
				span.SetAttributes(semconv.HTTPScheme(scheme))
				span.SetAttributes(semconv.HTTPRoute(routeStr))
				span.SetAttributes(semconv.ClientAddress(r.RemoteAddr))
				span.SetAttributes(semconv.HTTPMethod(r.Method))
				span.SetAttributes(semconv.HTTPUserAgent(r.UserAgent()))
				span.SetAttributes(semconv.HTTPRequestContentLength(int(r.ContentLength)))
				span.SetAttributes(semconv.HTTPURL(fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)))

				if requestId := middleware.GetReqID(ctx); requestId != "" {
					span.SetAttributes(attribute.Key("http.request_id").String(requestId))
				}

				span.SetName(fmt.Sprintf("%s:%s", r.Method, routeStr))

				if err := recover(); err != nil {
					span.SetAttributes(semconv.HTTPResponseStatusCode(500))
					span.SetStatus(codes.Error, fmt.Sprintf("%v", err))

					span.AddEvent("panic", oteltrace.WithAttributes(
						attribute.String("kind", "panic"),
						attribute.String("message", fmt.Sprintf("%v", err)),
						attribute.String("stack", string(debug.Stack())),
					))
					span.End()

					logger.ErrorContext(ctx, "Recover panic.", slog.String("error", fmt.Sprintf("%v", err)), slog.String("stack", string(debug.Stack())))
					http.Error(w, fmt.Sprintf("%#v", err), http.StatusInternalServerError)
				}
			}()

			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)

			if requestInfo.Metrics != nil {
				status := requestInfo.Metrics.Code
				written := requestInfo.Metrics.Written

				span.SetAttributes(semconv.HTTPResponseSize(int(written)))
				span.SetAttributes(semconv.HTTPResponseStatusCode(status))
				span.SetStatus(codes.Ok, http.StatusText(status))
			}
		})
	}
}

func handleMetricsAndLogs(r *http.Request, requestInfo *RequestInfo) {
	if requestInfo.Metrics != nil {
		ctx := oteltrace.ContextWithSpanContext(r.Context(), oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
			TraceID:    requestInfo.TraceId,
			SpanID:     requestInfo.SpanId,
			TraceFlags: requestInfo.TraceFlags,
			TraceState: requestInfo.TraceState,
			Remote:     false,
		}))

		path := chi.RouteContext(ctx).RoutePattern()
		status := requestInfo.Metrics.Code
		duration := requestInfo.Metrics.Duration
		written := requestInfo.Metrics.Written

		reqCount.Add(ctx, 1, metric.WithAttributes(
			attribute.String("response_code", strconv.Itoa(status)),
			attribute.String("request_method", r.Method),
			attribute.String("request_path", path),
		))
		reqDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(
			attribute.String("response_code", strconv.Itoa(status)),
			attribute.String("request_method", r.Method),
			attribute.String("request_path", path),
		))
		respSize.Record(ctx, written, metric.WithAttributes(
			attribute.String("response_code", strconv.Itoa(status)),
			attribute.String("request_method", r.Method),
			attribute.String("request_path", path),
		))

		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}

		if status >= 500 {
			logger.ErrorContext(
				ctx,
				"Request completed.",
				slog.String("http_scheme", scheme),
				slog.String("http_proto", r.Proto),
				slog.String("http_method", r.Method),
				slog.String("http_remote_address", r.RemoteAddr),
				slog.String("http_user_agent", strings.ReplaceAll(strings.ReplaceAll(r.UserAgent(), "\n", ""), "\r", "")),
				slog.String("http_url", fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)),
				slog.Duration("http_request_duration", duration),
				slog.Int("http_response_status_code", status),
				slog.Int64("http_response_size", written),
			)
		} else {
			logger.InfoContext(
				ctx,
				"Request completed.",
				slog.String("http_scheme", scheme),
				slog.String("http_proto", r.Proto),
				slog.String("http_method", r.Method),
				slog.String("http_remote_address", r.RemoteAddr),
				slog.String("http_user_agent", strings.ReplaceAll(strings.ReplaceAll(r.UserAgent(), "\n", ""), "\r", "")),
				slog.String("http_url", fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)),
				slog.Duration("http_request_duration", duration),
				slog.Int("http_response_status_code", status),
				slog.Int64("http_response_size", written),
			)
		}
	}
}
