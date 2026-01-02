package instrument

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	t.Run("should work with status code < 500", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		mux := http.NewServeMux()
		mux.Handle("/", Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(http.StatusText(http.StatusOK)))
		})))
		mux.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("should work with status code >= 500", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		mux := http.NewServeMux()
		mux.Handle("/", Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(http.StatusText(http.StatusInternalServerError)))
		})))
		mux.ServeHTTP(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("should handle panic", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		mux := http.NewServeMux()
		mux.Handle("/", Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test")
		})))
		mux.ServeHTTP(w, req)

		require.NotPanics(t, func() {
			mux.ServeHTTP(w, req)
		})
	})
}
