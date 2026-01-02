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
			slog.Int("http_response_status_code", m.Code),
			slog.String("http_request_method", r.Method),
			slog.String("http_route", route),
			slog.String("url_scheme", scheme),
			slog.String("url_path", r.URL.Path),
			slog.String("url_full", r.URL.String()),
			slog.String("user_agent_original", r.UserAgent()),
			slog.String("network_protocol_name", "http"),
			slog.String("network_protocol_version", fmt.Sprintf("%d.%d", r.ProtoMajor, r.ProtoMinor)),
			slog.String("server_address", serverAddress),
			slog.Int("server_port", serverPort),
			slog.String("client_address", clientAddress),
			slog.Int("client_port", clientPort),
			slog.String("network_peer_address", clientAddress),
			slog.Int("network_peer_port", clientPort),
			slog.Int64("http_request_body_size", r.ContentLength),
			slog.Int64("http_response_body_size", m.Written),
			slog.Duration("http_request_duration", m.Duration),
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
