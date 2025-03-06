package main

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

var handlerTracer = otel.Tracer("techdocs")

func echoHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := handlerTracer.Start(r.Context(), "echoHandler")
	defer span.End()

	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to dump request.", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
	}

	render.Status(r, http.StatusOK)
	render.Data(w, r, dump)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	_, span := handlerTracer.Start(r.Context(), "healthHandler")
	defer span.End()

	render.Status(r, http.StatusOK)
	render.Data(w, r, []byte("OK"))
}

func panicHandler(w http.ResponseWriter, r *http.Request) {
	_, span := handlerTracer.Start(r.Context(), "panicHandler")
	defer span.End()

	panic("panic test")
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := handlerTracer.Start(r.Context(), "statusHandler")
	defer span.End()

	randomStatusCodes := []int{200, 200, 200, 200, 200, 400, 500, 502, 503}

	statusString := r.URL.Query().Get("status")
	if statusString == "" || statusString == "random" {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(randomStatusCodes))))
		if err != nil {
			slog.ErrorContext(ctx, "Failed to generate random index.", slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		status := randomStatusCodes[index.Int64()]

		render.Status(r, status)
		render.Data(w, r, []byte(http.StatusText(status)))
		return
	}

	status, err := strconv.Atoi(statusString)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to parse 'status' parameter.", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	render.Status(r, status)
	render.Data(w, r, []byte(http.StatusText(status)))
}

func timeoutHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := handlerTracer.Start(r.Context(), "timeoutHandler")
	defer span.End()

	timeoutString := r.URL.Query().Get("timeout")
	if timeoutString == "" {
		err := fmt.Errorf("timeout parameter is missing")

		slog.ErrorContext(ctx, "Parameter 'timeout' is missing.", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	timeout, err := time.ParseDuration(timeoutString)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to parse 'timeout' parameter.", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	time.Sleep(timeout)

	render.Status(r, http.StatusOK)
	render.Data(w, r, []byte(http.StatusText(http.StatusOK)))
}

func headerSizeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := handlerTracer.Start(r.Context(), "headerSizeHandler")
	defer span.End()

	headerSizeString := r.URL.Query().Get("size")
	if headerSizeString == "" {
		err := fmt.Errorf("size parameter is missing")

		slog.ErrorContext(ctx, "Parameter 'size' is missing.", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	size, err := strconv.Atoi(headerSizeString)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to parse 'size' parameter.", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Add("X-Header-Size", strings.Repeat("0", size))
	render.Status(r, http.StatusOK)
	render.Data(w, r, []byte(http.StatusText(http.StatusOK)))
}
