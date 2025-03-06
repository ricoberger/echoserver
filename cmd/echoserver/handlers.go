package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

var httpClient = &http.Client{
	Transport: otelhttp.NewTransport(
		http.DefaultTransport,
		otelhttp.WithClientTrace(func(ctx context.Context) *httptrace.ClientTrace {
			return otelhttptrace.NewClientTrace(ctx, otelhttptrace.WithoutSubSpans())
		}),
	),
}

type Request struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
}

func requestHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := handlerTracer.Start(r.Context(), "requestHandler")
	defer span.End()

	var request Request
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		slog.ErrorContext(ctx, "Failed to decode request body.", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	req, err := http.NewRequestWithContext(ctx, request.Method, request.URL, strings.NewReader(request.Body))
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create http request.", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for key, value := range request.Headers {
		req.Header.Add(key, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to do http request.", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to read reespons body.", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	render.Status(r, resp.StatusCode)
	render.Data(w, r, body)
}
