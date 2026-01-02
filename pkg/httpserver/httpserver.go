package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"github.com/ricoberger/echoserver/pkg/httpserver/middleware/instrument"
	"github.com/ricoberger/echoserver/pkg/httpserver/middleware/requestid"

	"github.com/felixge/fgprof"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
)

var (
	tracer = otel.Tracer("httpserver")
)

type Config struct {
	Address string `env:"ADDRESS" default:":8080" help:"The address where the HTTP server should listen on."`
}

type Server interface {
	Start()
	Stop()
}

type server struct {
	server *http.Server
}

func (s *server) Start() {
	slog.Info("Start server...", slog.String("address", s.server.Addr))

	if err := s.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		slog.Error("Server died unexpected.", slog.Any("error", err))
	}
}

func (s *server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown error.", slog.Any("error", err))
	}
}

func New(config Config) Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", echoHandler)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/panic", panicHandler)
	mux.HandleFunc("/status", statusHandler)
	mux.HandleFunc("/timeout", timeoutHandler)
	mux.HandleFunc("/headersize", headerSizeHandler)
	mux.HandleFunc("/request", requestHandler)
	mux.HandleFunc("/fibonacci", fibonacciHandler)
	mux.HandleFunc("/websocket", websocketHandler)
	mux.HandleFunc("/debug/pprof", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	mux.Handle("/debug/pprof/trace", pprof.Handler("trace"))
	mux.Handle("/debug/pprof/fgprof", fgprof.Handler())

	if os.Getenv("OTEL_METRICS_EXPORTER") == "prometheus" {
		// To view exemplars, the following cURL command can be used:
		// curl -H 'Accept: application/openmetrics-text' 'http://localhost:8080/metrics'
		mux.Handle("/metrics", promhttp.InstrumentMetricHandler(
			prometheus.DefaultRegisterer,
			promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true}),
		))
	}

	return &server{
		server: &http.Server{
			Addr:              config.Address,
			Handler:           requestid.Handler(instrument.Handler(mux)),
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}
