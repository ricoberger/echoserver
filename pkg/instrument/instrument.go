package instrument

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/felixge/httpsnoop"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type ctxKeyRequestInfo int

const RequestInfoKey ctxKeyRequestInfo = 0

type RequestInfo struct {
	Metrics *httpsnoop.Metrics
}

var (
	reqCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "echoserver",
		Name:      "http_requests_total",
		Help:      "Number of HTTP requests processed, partitioned by status code, method and path.",
	}, []string{"response_code", "request_method", "request_path"})

	reqDurationSum = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  "echoserver",
		Name:       "http_request_duration_seconds",
		Help:       "Latency of HTTP requests processed, partitioned by status code, method and path.",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.95: 0.005, 0.99: 0.001},
	}, []string{"response_code", "request_method", "request_path"})

	respSizeSum = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  "echoserver",
		Name:       "http_response_size_bytes",
		Help:       "Size of HTTP responses, partitioned by status code, method and path.",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.95: 0.005, 0.99: 0.001},
	}, []string{"response_code", "request_method", "request_path"})
)

func Handler() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var requestInfo = &RequestInfo{}
			r = r.WithContext(context.WithValue(r.Context(), RequestInfoKey, requestInfo))

			handler := handleInstrumentation(requestInfo)(next)

			requestInfo.Metrics = &httpsnoop.Metrics{
				Code: http.StatusOK,
			}
			requestInfo.Metrics.CaptureMetrics(w, func(ww http.ResponseWriter) {
				handler.ServeHTTP(ww, r)
			})
		})
	}
}

func handleInstrumentation(requestInfo *RequestInfo) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			ctx, span := otel.Tracer("http.request").Start(ctx, "http.request", oteltrace.WithSpanKind(oteltrace.SpanKindServer))
			defer span.End()

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
					path := chi.RouteContext(r.Context()).RoutePattern()
					duration := requestInfo.Metrics.Duration
					written := requestInfo.Metrics.Written

					reqCount.WithLabelValues(strconv.Itoa(http.StatusInternalServerError), r.Method, path).Inc()
					reqDurationSum.WithLabelValues(strconv.Itoa(http.StatusInternalServerError), r.Method, path).Observe(duration.Seconds())
					respSizeSum.WithLabelValues(strconv.Itoa(http.StatusInternalServerError), r.Method, path).Observe(float64(written))

					span.SetAttributes(semconv.HTTPResponseStatusCode(500))
					span.SetStatus(codes.Error, fmt.Sprintf("%v", err))

					span.AddEvent("panic", oteltrace.WithAttributes(
						attribute.String("kind", "panic"),
						attribute.String("message", fmt.Sprintf("%v", err)),
						attribute.String("stack", string(debug.Stack())),
					))
					span.End()

					slog.ErrorContext(
						r.Context(),
						"Request completed.",
						slog.String("requestScheme", scheme),
						slog.String("requestProto", r.Proto),
						slog.String("requestMethod", r.Method),
						slog.String("requestAddr", r.RemoteAddr),
						slog.String("requestUserAgent", strings.ReplaceAll(strings.ReplaceAll(r.UserAgent(), "\n", ""), "\r", "")),
						slog.String("requestURI", fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)),
						slog.Int("responseStatus", 500),
						slog.String("error", fmt.Sprintf("%v", err)),
						slog.String("stack", string(debug.Stack())),
					)

					panic(err)
				}
			}()

			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)

			if requestInfo.Metrics != nil {
				path := chi.RouteContext(r.Context()).RoutePattern()
				status := requestInfo.Metrics.Code
				duration := requestInfo.Metrics.Duration
				written := requestInfo.Metrics.Written

				reqCount.WithLabelValues(strconv.Itoa(status), r.Method, path).Inc()
				reqDurationSum.WithLabelValues(strconv.Itoa(status), r.Method, path).Observe(duration.Seconds())
				respSizeSum.WithLabelValues(strconv.Itoa(status), r.Method, path).Observe(float64(written))

				span.SetAttributes(semconv.HTTPResponseSize(int(written)))
				span.SetAttributes(semconv.HTTPResponseStatusCode(status))

				if status >= 500 {
					slog.ErrorContext(
						r.Context(),
						"Request completed.",
						slog.String("requestScheme", scheme),
						slog.String("requestProto", r.Proto),
						slog.String("requestMethod", r.Method),
						slog.String("requestAddr", r.RemoteAddr),
						slog.String("requestUserAgent", strings.ReplaceAll(strings.ReplaceAll(r.UserAgent(), "\n", ""), "\r", "")),
						slog.String("requestURI", fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)),
						slog.Int("responseStatus", status),
					)
				} else {
					slog.InfoContext(
						r.Context(),
						"Request completed.",
						slog.String("requestScheme", scheme),
						slog.String("requestProto", r.Proto),
						slog.String("requestMethod", r.Method),
						slog.String("requestAddr", r.RemoteAddr),
						slog.String("requestUserAgent", strings.ReplaceAll(strings.ReplaceAll(r.UserAgent(), "\n", ""), "\r", "")),
						slog.String("requestURI", fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)),
						slog.Int("responseStatus", status),
					)
				}
			}
		})
	}
}
