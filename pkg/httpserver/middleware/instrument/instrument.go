package instrument

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/ricoberger/echoserver/pkg/httpserver/middleware/requestid"

	"github.com/felixge/httpsnoop"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.opentelemetry.io/otel/trace"
)

func Handler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		m := httpsnoop.CaptureMetrics(next, w, r)

		ctx := r.Context()

		span := trace.SpanFromContext(ctx)
		span.SetAttributes(attribute.String("http.request.id", requestid.Get(ctx)))

		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		serverAddress, serverPortStr, _ := net.SplitHostPort(r.Host)
		clientAddress, clientPortStr, _ := net.SplitHostPort(r.RemoteAddr)
		serverPort := parsePort(serverPortStr)
		clientPort := parsePort(clientPortStr)
		route := GetRoute(r)

		slog.InfoContext(
			ctx,
			"Request completed.",
			slog.Int(string(semconv.HTTPResponseStatusCodeKey), m.Code),
			slog.String(string(semconv.HTTPRequestMethodKey), r.Method),
			slog.String(string(semconv.HTTPRouteKey), route),
			slog.String(string(semconv.URLSchemeKey), scheme),
			slog.String(string(semconv.URLPathKey), r.URL.Path),
			slog.String(string(semconv.URLFullKey), r.URL.String()),
			slog.String(string(semconv.UserAgentOriginalKey), r.UserAgent()),
			slog.String(string(semconv.NetworkProtocolNameKey), "http"),
			slog.String(string(semconv.NetworkProtocolVersionKey), fmt.Sprintf("%d.%d", r.ProtoMajor, r.ProtoMinor)),
			slog.String(string(semconv.ServerAddressKey), serverAddress),
			slog.Int(string(semconv.ServerPortKey), serverPort),
			slog.String(string(semconv.ClientAddressKey), clientAddress),
			slog.Int(string(semconv.ClientPortKey), clientPort),
			slog.String(string(semconv.NetworkPeerAddressKey), clientAddress),
			slog.Int(string(semconv.NetworkPeerPortKey), clientPort),
			slog.Int64(string(semconv.HTTPRequestBodySizeKey), r.ContentLength),
			slog.Int64(string(semconv.HTTPResponseBodySizeKey), m.Written),
			slog.Duration("http.request.duration", m.Duration),
		)
	}

	return http.HandlerFunc(fn)
}

func GetRoute(r *http.Request) string {
	if r.Pattern != "" {
		parts := strings.SplitN(r.Pattern, " ", 2)
		if len(parts) == 2 {
			return parts[1]
		}
		return r.Pattern
	}
	return r.URL.Path
}

func parsePort(port string) int {
	p, _ := strconv.ParseInt(port, 10, 64)
	return int(p)
}
