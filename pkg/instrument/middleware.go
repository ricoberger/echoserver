package instrument

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/felixge/httpsnoop"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.38.0"
	httpconv "go.opentelemetry.io/otel/semconv/v1.38.0/httpconv"
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
	meter  = otel.Meter("instrument")

	reqCount    metric.Int64Counter
	reqDuration httpconv.ServerRequestDuration
	reqSize     httpconv.ServerRequestBodySize
	respSize    httpconv.ServerResponseBodySize
)

func init() {
	reqCount, _ = meter.Int64Counter(
		"http.server.request.total",
		metric.WithDescription("Number of HTTP requests processed, partitioned by status code, method and path."),
	)
	reqDuration, _ = httpconv.NewServerRequestDuration(meter, metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10))
	reqSize, _ = httpconv.NewServerRequestBodySize(meter)
	respSize, _ = httpconv.NewServerResponseBodySize(meter)
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

			defer func() {
				scheme := "http"
				if r.TLS != nil {
					scheme = "https"
				}
				serverAddress, serverPortStr, _ := net.SplitHostPort(r.Host)
				clientAddress, clientPortStr, _ := net.SplitHostPort(r.Host)
				serverPort := parsePort(serverPortStr)
				clientPort := parsePort(clientPortStr)
				route := chi.RouteContext(ctx).RoutePattern()

				span.SetAttributes(semconv.HTTPRequestMethodKey.String(r.Method))
				span.SetAttributes(semconv.HTTPRoute(route))
				span.SetAttributes(semconv.URLScheme(scheme))
				span.SetAttributes(semconv.NetworkProtocolName("http"))
				span.SetAttributes(semconv.NetworkProtocolVersion(fmt.Sprintf("%d.%d", r.ProtoMajor, r.ProtoMinor)))
				span.SetAttributes(semconv.ServerAddress(serverAddress))
				span.SetAttributes(semconv.ServerPort(serverPort))
				span.SetAttributes(semconv.ClientAddress(clientAddress))
				span.SetAttributes(semconv.ClientPort(clientPort))
				span.SetAttributes(semconv.UserAgentOriginal(r.UserAgent()))
				span.SetAttributes(attribute.Key(semconv.HTTPRequestBodySizeKey).Int64(r.ContentLength))
				span.SetAttributes(semconv.URLFull(fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)))

				if requestId := middleware.GetReqID(ctx); requestId != "" {
					span.SetAttributes(attribute.Key("http.request_id").String(requestId))
				}

				span.SetName(fmt.Sprintf("%s:%s", r.Method, route))

				if err := recover(); err != nil {
					span.SetAttributes(semconv.HTTPResponseStatusCode(500))
					span.SetStatus(codes.Error, fmt.Sprintf("%v", err))

					span.AddEvent("panic", oteltrace.WithAttributes(
						attribute.String("kind", "panic"),
						attribute.String("message", fmt.Sprintf("%v", err)),
						attribute.String("stack", string(debug.Stack())),
					))
					span.End()

					slog.ErrorContext(ctx, "Recover panic.", slog.String("error", fmt.Sprintf("%v", err)), slog.String("stack", string(debug.Stack())))
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

		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		serverAddress, serverPortStr, _ := net.SplitHostPort(r.Host)
		clientAddress, clientPortStr, _ := net.SplitHostPort(r.Host)
		serverPort := parsePort(serverPortStr)
		clientPort := parsePort(clientPortStr)
		route := chi.RouteContext(ctx).RoutePattern()
		status := requestInfo.Metrics.Code
		duration := requestInfo.Metrics.Duration
		written := requestInfo.Metrics.Written

		reqCount.Add(ctx, 1, metric.WithAttributes(
			semconv.HTTPResponseStatusCode(status),
			semconv.HTTPRequestMethodKey.String(r.Method),
			semconv.HTTPRoute(route),
			semconv.URLScheme(scheme),
			semconv.NetworkProtocolName("http"),
			semconv.NetworkProtocolVersion(fmt.Sprintf("%d.%d", r.ProtoMajor, r.ProtoMinor)),
			semconv.ServerAddress(serverAddress),
			semconv.ServerPort(serverPort),
		))
		reqDuration.Record(ctx, duration.Seconds(), getMethod(r.Method), scheme,
			semconv.HTTPResponseStatusCode(status),
			semconv.HTTPRoute(route),
			semconv.NetworkProtocolName("http"),
			semconv.NetworkProtocolVersion(fmt.Sprintf("%d.%d", r.ProtoMajor, r.ProtoMinor)),
			semconv.ServerAddress(serverAddress),
			semconv.ServerPort(serverPort),
		)
		reqSize.Record(ctx, r.ContentLength, getMethod(r.Method), scheme,
			semconv.HTTPResponseStatusCode(status),
			semconv.HTTPRoute(route),
			semconv.NetworkProtocolName("http"),
			semconv.NetworkProtocolVersion(fmt.Sprintf("%d.%d", r.ProtoMajor, r.ProtoMinor)),
			semconv.ServerAddress(serverAddress),
			semconv.ServerPort(serverPort),
		)
		respSize.Record(ctx, written, getMethod(r.Method), scheme,
			semconv.HTTPResponseStatusCode(status),
			semconv.HTTPRoute(route),
			semconv.NetworkProtocolName("http"),
			semconv.NetworkProtocolVersion(fmt.Sprintf("%d.%d", r.ProtoMajor, r.ProtoMinor)),
			semconv.ServerAddress(serverAddress),
			semconv.ServerPort(serverPort),
		)

		if status >= 500 {
			slog.ErrorContext(
				ctx,
				"Request completed.",
				slog.Int("http_response_status_code", status),
				slog.String("http_request_method", r.Method),
				slog.String("http_route", route),
				slog.String("user_agent_original", strings.ReplaceAll(strings.ReplaceAll(r.UserAgent(), "\n", ""), "\r", "")),
				slog.String("http_remote_address", r.RemoteAddr),
				slog.String("http_scheme", scheme),
				slog.String("network_protocol_name", "http"),
				slog.String("network_protocol_version", fmt.Sprintf("%d.%d", r.ProtoMajor, r.ProtoMinor)),
				slog.String("server_address", serverAddress),
				slog.Int("server_port", serverPort),
				slog.String("client_address", clientAddress),
				slog.Int("client_port", clientPort),
				slog.Int64("http_request_body_size", r.ContentLength),
				slog.String("url_full", fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)),
				slog.Int64("http_response_body_size", written),
				slog.Duration("http_request_duration", duration),
			)
		} else {
			slog.InfoContext(
				ctx,
				"Request completed.",
				slog.Int("http_response_status_code", status),
				slog.String("http_request_method", r.Method),
				slog.String("http_route", route),
				slog.String("user_agent_original", strings.ReplaceAll(strings.ReplaceAll(r.UserAgent(), "\n", ""), "\r", "")),
				slog.String("http_remote_address", r.RemoteAddr),
				slog.String("http_scheme", scheme),
				slog.String("network_protocol_name", "http"),
				slog.String("network_protocol_version", fmt.Sprintf("%d.%d", r.ProtoMajor, r.ProtoMinor)),
				slog.String("server_address", serverAddress),
				slog.Int("server_port", serverPort),
				slog.String("client_address", clientAddress),
				slog.Int("client_port", clientPort),
				slog.Int64("http_request_body_size", r.ContentLength),
				slog.String("url_full", fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)),
				slog.Int64("http_response_body_size", written),
				slog.Duration("http_request_duration", duration),
			)
		}
	}
}

func getMethod(method string) httpconv.RequestMethodAttr {
	var methodLookup = map[string]httpconv.RequestMethodAttr{
		http.MethodConnect: httpconv.RequestMethodConnect,
		http.MethodDelete:  httpconv.RequestMethodDelete,
		http.MethodGet:     httpconv.RequestMethodGet,
		http.MethodHead:    httpconv.RequestMethodHead,
		http.MethodOptions: httpconv.RequestMethodOptions,
		http.MethodPatch:   httpconv.RequestMethodPatch,
		http.MethodPost:    httpconv.RequestMethodPost,
		http.MethodPut:     httpconv.RequestMethodPut,
		http.MethodTrace:   httpconv.RequestMethodTrace,
	}

	if method == "" {
		return httpconv.RequestMethodGet
	}
	if attr, ok := methodLookup[method]; ok {
		return attr
	}
	return httpconv.RequestMethodGet
}

func parsePort(port string) int {
	p, _ := strconv.ParseInt(port, 10, 64)
	return int(p)
}
