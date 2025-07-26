package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ricoberger/echoserver/pkg/instrument"
	"github.com/ricoberger/echoserver/pkg/instrument/logger"
	"github.com/ricoberger/echoserver/pkg/instrument/tracer"
	"github.com/ricoberger/echoserver/pkg/recoverer"
	"github.com/ricoberger/echoserver/pkg/version"

	"github.com/alecthomas/kong"
	"github.com/felixge/fgprof"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Cli struct {
	Address string `env:"ADDRESS" default:":8080" help:"The address where the server should listen on."`

	Log    logger.Config `embed:"" prefix:"log." envprefix:"LOG_"`
	Tracer tracer.Config `embed:"" prefix:"tracer." envprefix:"TRACER_"`
}

func main() {
	var cli Cli

	ctx := kong.Parse(&cli, kong.Name("echoserver"))
	ctx.FatalIfErrorf(ctx.Error)
	ctx.FatalIfErrorf(cli.run())
}

func (c *Cli) run() error {
	ctx := context.Background()

	logger := logger.New(c.Log)
	logger.InfoContext(ctx, "Version information.", "version", slog.GroupValue(version.Info()...))
	logger.InfoContext(ctx, "Build information.", "build", slog.GroupValue(version.BuildContext()...))

	tracer, err := tracer.New(c.Tracer)
	if err != nil {
		return err
	}
	defer tracer.Shutdown()

	router := chi.NewRouter()
	router.Use(recoverer.Handler)
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
	router.Handle("/metrics", promhttp.Handler())
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

	server := &http.Server{
		Addr:              c.Address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			slog.ErrorContext(ctx, "Server died unexpected.", slog.Any("error", err))
		}
		slog.ErrorContext(ctx, "Server stopped.")
	}()

	// All components should be terminated gracefully. For that we are listen
	// for the SIGINT and SIGTERM signals and try to gracefully shutdown the
	// started components. This ensures that established connections or tasks
	// are not interrupted.
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	logger.DebugContext(ctx, "Start listining for SIGINT and SIGTERM signal.")
	<-done
	logger.InfoContext(ctx, "Shutdown started.")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.ErrorContext(ctx, "HTTP server shutdown error", slog.Any("error", err))
		return err
	}

	logger.InfoContext(ctx, "Shutdown done.")

	return nil
}
