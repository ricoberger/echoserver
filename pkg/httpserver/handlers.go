package httpserver

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"github.com/ricoberger/echoserver/pkg/httpserver/middleware/requestid"
	"github.com/ricoberger/echoserver/pkg/simulate"

	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

func echoHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "echoHandler")
	defer span.End()

	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		handleError(ctx, w, span, http.StatusInternalServerError, "Failed to dump request.", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	//nolint:gosec
	w.Write(dump)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	_, span := tracer.Start(r.Context(), "healthHandler")
	defer span.End()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func panicHandler(w http.ResponseWriter, r *http.Request) {
	_, span := tracer.Start(r.Context(), "panicHandler")
	defer span.End()

	panic("panic test")
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "statusHandler")
	defer span.End()
	span.SetAttributes(attribute.Key("http.parameter.status").String(r.URL.Query().Get("status")))
	slog.DebugContext(ctx, "Handling status request.", slog.String("status", r.URL.Query().Get("status")))

	randomStatusCodes := []int{200, 200, 200, 200, 200, 400, 500, 502, 503}

	statusString := r.URL.Query().Get("status")
	if statusString == "" || statusString == "random" {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(randomStatusCodes))))
		if err != nil {
			handleError(ctx, w, span, http.StatusInternalServerError, "Failed to generate random index.", err)
			return
		}

		status := randomStatusCodes[index.Int64()]

		w.WriteHeader(status)
		w.Write([]byte(http.StatusText(status)))
		return
	}

	status, err := strconv.Atoi(statusString)
	if err != nil {
		handleError(ctx, w, span, http.StatusBadRequest, "Invalid 'status' parameter.", fmt.Errorf("failed to parse 'status' parameter: %w", err))
		return
	}

	w.WriteHeader(status)
	w.Write([]byte(http.StatusText(status)))
}

func timeoutHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "timeoutHandler")
	defer span.End()
	span.SetAttributes(attribute.Key("http.parameter.timeout").String(r.URL.Query().Get("timeout")))
	span.SetAttributes(attribute.Key("http.parameter.flush").String(r.URL.Query().Get("flush")))
	slog.DebugContext(ctx, "Handling timeout request.", slog.String("timeout", r.URL.Query().Get("timeout")), slog.String("flush", r.URL.Query().Get("flush")))

	timeout, err := parseRequiredDurationParam(r, "timeout")
	if err != nil {
		handleError(ctx, w, span, http.StatusBadRequest, "Invalid 'timeout' parameter.", err)
		return
	}

	if flushString := r.URL.Query().Get("flush"); flushString != "" {
		if flush, err := time.ParseDuration(flushString); err == nil && flush > 0 {
			done := make(chan bool)

			go func() {
				ticker := time.NewTicker(flush)
				defer ticker.Stop()

				for {
					select {
					case <-done:
						return
					case <-ticker.C:
						if f, ok := w.(http.Flusher); ok {
							span.AddEvent("Flush")
							w.Write([]byte(http.StatusText(http.StatusProcessing) + "\n"))
							f.Flush()
						}
					}
				}
			}()

			defer func() {
				done <- true
			}()
		}
	}

	select {
	case <-ctx.Done():
		w.Write([]byte(http.StatusText(http.StatusBadRequest)))
		return
	case <-time.After(timeout):
	}

	w.Write([]byte(http.StatusText(http.StatusOK)))
}

func headerSizeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "headerSizeHandler")
	defer span.End()
	span.SetAttributes(attribute.Key("http.parameter.size").String(r.URL.Query().Get("size")))
	slog.DebugContext(ctx, "Handling header size request.", slog.String("size", r.URL.Query().Get("size")))

	size, err := parseRequiredIntParam(r, "size")
	if err != nil {
		handleError(ctx, w, span, http.StatusBadRequest, "Invalid 'size' parameter.", err)
		return
	}

	w.Header().Add("X-Header-Size", strings.Repeat("0", size))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

type Request struct {
	Method            string             `json:"method"`
	URL               string             `json:"url"`
	Body              string             `json:"body"`
	Headers           map[string]string  `json:"headers"`
	HTTPClientOptions *HTTPClientOptions `json:"httpClientOptions"`
}

func requestHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "requestHandler")
	defer span.End()

	var request Request
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		handleError(ctx, w, span, http.StatusBadRequest, "Failed to decode request body.", err)
		return
	}

	span.SetAttributes(attribute.Key("http.parameter.method").String(request.Method))
	span.SetAttributes(attribute.Key("http.parameter.url").String(request.URL))
	span.SetAttributes(attribute.Key("http.parameter.body").String(request.Body))
	slog.DebugContext(ctx, "Handling request request.", slog.String("method", request.Method), slog.String("url", request.URL), slog.String("body", request.Body))

	req, err := http.NewRequestWithContext(ctx, request.Method, request.URL, strings.NewReader(request.Body))
	if err != nil {
		handleError(ctx, w, span, http.StatusBadRequest, "Failed to create http request.", err)
		return
	}

	for key, value := range request.Headers {
		req.Header.Add(key, value)
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	if requestId := requestid.Get(ctx); requestId != "" {
		req.Header.Set(requestid.RequestIDHeader, requestId)
	}

	resp, err := getHTTPClient(request.HTTPClientOptions).Do(req)
	if err != nil {
		handleError(ctx, w, span, http.StatusBadRequest, "Failed to do http request.", err)
		return
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		handleError(ctx, w, span, http.StatusBadRequest, "Failed to read response body.", err)
		return
	}

	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func fibonacci(n uint64) (*big.Int, *big.Int) {
	if n == 0 {
		return big.NewInt(0), big.NewInt(1)
	}
	a, b := fibonacci(n / 2)
	c := big.NewInt(0).Mul(a, big.NewInt(0).Sub(big.NewInt(0).Mul(b, big.NewInt(2)), a))
	d := big.NewInt(0).Add(big.NewInt(0).Mul(a, a), big.NewInt(0).Mul(b, b))
	if n%2 == 0 {
		return c, d
	}
	return d, big.NewInt(0).Add(d, c)
}

func fibonacciHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "fibonacciHandler")
	defer span.End()
	span.SetAttributes(attribute.Key("http.parameter.n").String(r.URL.Query().Get("n")))
	slog.DebugContext(ctx, "Handling fibonacci request.", slog.String("n", r.URL.Query().Get("n")))

	nString := r.URL.Query().Get("n")
	if nString == "" {
		handleError(ctx, w, span, http.StatusBadRequest, "Invalid 'n' parameter.", fmt.Errorf("n parameter is missing"))
		return
	}

	n, err := strconv.ParseUint(nString, 10, 64)
	if err != nil {
		handleError(ctx, w, span, http.StatusBadRequest, "Invalid 'n' parameter.", fmt.Errorf("failed to parse 'n' parameter: %w", err))
		return
	}

	span.AddEvent("Start calculation")
	res, _ := fibonacci(n)
	span.AddEvent("Calculation completed")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(res.String()))
}

func simulateHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "simulateHandler")
	defer span.End()
	span.SetAttributes(attribute.Key("http.parameter.type").String(r.URL.Query().Get("type")))
	span.SetAttributes(attribute.Key("http.parameter.duration").String(r.URL.Query().Get("duration")))
	slog.DebugContext(ctx, "Handling simulate request.", slog.String("type", r.URL.Query().Get("type")), slog.String("duration", r.URL.Query().Get("duration")))

	simulationType := r.URL.Query().Get("type")
	if simulationType == "" {
		handleError(ctx, w, span, http.StatusBadRequest, "Invalid 'type' parameter.", fmt.Errorf("type parameter is missing"))
		return
	}

	duration, err := parseRequiredDurationParam(r, "duration")
	if err != nil {
		handleError(ctx, w, span, http.StatusBadRequest, "Invalid 'duration' parameter.", err)
		return
	}

	var message string
	switch simulationType {
	case "cpu":
		simulate.CPU(ctx, duration)
		message = fmt.Sprintf("simulated %q for %s", simulationType, duration)
	case "memory":
		size, err := parseRequiredIntParam(r, "size")
		if err != nil {
			handleError(ctx, w, span, http.StatusBadRequest, "Invalid 'size' parameter.", err)
			return
		}
		span.SetAttributes(attribute.Key("http.parameter.size").Int(size))
		simulate.Memory(ctx, duration, size)
		message = fmt.Sprintf("simulated %q for %s (%d bytes)", simulationType, duration, size)
	case "goroutines":
		count, err := parseRequiredIntParam(r, "count")
		if err != nil {
			handleError(ctx, w, span, http.StatusBadRequest, "Invalid 'count' parameter.", err)
			return
		}
		span.SetAttributes(attribute.Key("http.parameter.count").Int(count))
		simulate.Goroutines(ctx, duration, count)
		message = fmt.Sprintf("simulated %q for %s (%d goroutines)", simulationType, duration, count)
	case "mutex":
		workers, err := parseRequiredIntParam(r, "workers")
		if err != nil {
			handleError(ctx, w, span, http.StatusBadRequest, "Invalid 'workers' parameter.", err)
			return
		}
		span.SetAttributes(attribute.Key("http.parameter.workers").Int(workers))
		simulate.Mutex(ctx, duration, workers)
		message = fmt.Sprintf("simulated %q for %s (%d workers)", simulationType, duration, workers)
	case "block":
		workers, err := parseRequiredIntParam(r, "workers")
		if err != nil {
			handleError(ctx, w, span, http.StatusBadRequest, "Invalid 'workers' parameter.", err)
			return
		}
		span.SetAttributes(attribute.Key("http.parameter.workers").Int(workers))
		simulate.Block(ctx, duration, workers)
		message = fmt.Sprintf("simulated %q for %s (%d workers)", simulationType, duration, workers)
	default:
		handleError(ctx, w, span, http.StatusBadRequest, "Unknown simulation type.", fmt.Errorf("unknown simulation type %q", simulationType))
		return
	}

	w.WriteHeader(http.StatusOK)
	//nolint:gosec
	w.Write([]byte(message))
}

var upgrader = websocket.Upgrader{}

func websocketHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "websocketHandler")
	defer span.End()

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to upgrade connection.", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}
	defer c.Close()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	c.SetReadDeadline(time.Now().Add(30 * time.Second))

	c.SetPongHandler(func(string) error {
		slog.DebugContext(ctx, "Received pong from client.")
		span.AddEvent("Received pong from client")
		c.SetReadDeadline(time.Now().Add(30 * time.Second))
		return nil
	})

	go func() {
		for {
			<-ticker.C

			slog.DebugContext(ctx, "Sent ping to client.")
			span.AddEvent("Sent ping to client")

			if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.ErrorContext(ctx, "Failed to send ping.", slog.Any("error", err))
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return
			}
		}
	}()

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
				slog.ErrorContext(ctx, "Failed to read message.", slog.Any("error", err))
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			break
		}

		slog.DebugContext(ctx, "Received message.", slog.String("message", string(message)))
		span.AddEvent(fmt.Sprintf("Received message: %s", string(message)))

		err = c.WriteMessage(mt, message)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to write message.", slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			break
		}
	}
}
