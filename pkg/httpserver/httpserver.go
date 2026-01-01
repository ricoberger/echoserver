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

	"github.com/felixge/fgprof"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(instrument.Handler())
	router.HandleFunc("/", echoHandler)
	router.HandleFunc("/health", healthHandler)
	router.HandleFunc("/panic", panicHandler)
	router.HandleFunc("/status", statusHandler)
	router.HandleFunc("/timeout", timeoutHandler)
	router.HandleFunc("/headersize", headerSizeHandler)
	router.HandleFunc("/request", requestHandler)
	router.HandleFunc("/fibonacci", fibonacciHandler)
	router.HandleFunc("/websocket", websocketHandler)
	router.HandleFunc("/debug/pprof", pprof.Index)
	router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	router.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	router.Handle("/debug/pprof/block", pprof.Handler("block"))
	router.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	router.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	router.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	router.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	router.Handle("/debug/pprof/trace", pprof.Handler("trace"))
	router.Handle("/debug/pprof/fgprof", fgprof.Handler())

	if os.Getenv("OTEL_METRICS_EXPORTER") == "prometheus" {
		// To view exemplars, the following cURL command can be used:
		// curl -H 'Accept: application/openmetrics-text' 'http://localhost:8080/metrics'
		router.Handle("/metrics", promhttp.InstrumentMetricHandler(
			prometheus.DefaultRegisterer,
			promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true}),
		))
	}

	return &server{
		server: &http.Server{
			Addr:              config.Address,
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}
