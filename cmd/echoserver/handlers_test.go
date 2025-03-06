package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func TestEchoHandler(t *testing.T) {
	t.Run("should dump get request", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		router := chi.NewRouter()
		router.HandleFunc("/", echoHandler)
		router.ServeHTTP(w, req)

		body, err := io.ReadAll(w.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, w.Code)
		require.Contains(t, string(body), "GET")
		require.Contains(t, string(body), "HTTP")
	})

	t.Run("should dump post request", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", bytes.NewBuffer([]byte("test body")))
		w := httptest.NewRecorder()

		router := chi.NewRouter()
		router.HandleFunc("/", echoHandler)
		router.ServeHTTP(w, req)

		body, err := io.ReadAll(w.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, w.Code)
		require.Contains(t, string(body), "POST")
		require.Contains(t, string(body), "HTTP")
		require.Contains(t, string(body), "test body")
	})
}

func TestHealthHandler(t *testing.T) {
	t.Run("should return ok", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		router := chi.NewRouter()
		router.HandleFunc("/", healthHandler)
		router.ServeHTTP(w, req)

		body, err := io.ReadAll(w.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusText(http.StatusOK), string(body))
	})
}

func TestStatusHandler(t *testing.T) {
	t.Run("should return random status code", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		router := chi.NewRouter()
		router.HandleFunc("/", statusHandler)
		router.ServeHTTP(w, req)

		require.Contains(t, []int{200, 400, 500, 502, 503}, w.Code)
	})

	t.Run("should return specific status code", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/?status=300", nil)
		w := httptest.NewRecorder()

		router := chi.NewRouter()
		router.HandleFunc("/", statusHandler)
		router.ServeHTTP(w, req)

		require.Equal(t, 300, w.Code)
	})

	t.Run("should return error for invalid status code", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/?status=invalid", nil)
		w := httptest.NewRecorder()

		router := chi.NewRouter()
		router.HandleFunc("/", statusHandler)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestTimeouthandler(t *testing.T) {
	t.Run("should return after specified timeout", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/?timeout=1s", nil)
		w := httptest.NewRecorder()

		router := chi.NewRouter()
		router.HandleFunc("/", timeoutHandler)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("should return error when timeout parameter is missing", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/?timeout=", nil)
		w := httptest.NewRecorder()

		router := chi.NewRouter()
		router.HandleFunc("/", timeoutHandler)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("should return error when timeout parameter is invalid", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/?timeout=invalid", nil)
		w := httptest.NewRecorder()

		router := chi.NewRouter()
		router.HandleFunc("/", timeoutHandler)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHeaderSizeHandler(t *testing.T) {
	t.Run("should return header with the specified size", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/?size=10", nil)
		w := httptest.NewRecorder()

		router := chi.NewRouter()
		router.HandleFunc("/", headerSizeHandler)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, 10, len(w.Header().Get("X-Header-Size")))
	})

	t.Run("should return error when size parameter is missing", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/?size=", nil)
		w := httptest.NewRecorder()

		router := chi.NewRouter()
		router.HandleFunc("/", headerSizeHandler)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("should return error when size parameter is invalid", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/?size=invalid", nil)
		w := httptest.NewRecorder()

		router := chi.NewRouter()
		router.HandleFunc("/", headerSizeHandler)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
