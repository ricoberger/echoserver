package instrument

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	t.Run("should work with status code < 500", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		router := chi.NewRouter()
		router.Use(middleware.RequestID)
		router.Use(Handler())
		router.Get("/", func(w http.ResponseWriter, r *http.Request) {
			render.Status(r, http.StatusOK)
			render.JSON(w, r, nil)
		})
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("should work with status code >= 500", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		router := chi.NewRouter()
		router.Use(middleware.RequestID)
		router.Use(Handler())
		router.Get("/", func(w http.ResponseWriter, r *http.Request) {
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, nil)
		})
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("should handle panic", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		router := chi.NewRouter()
		router.Use(Handler())
		router.Get("/", func(w http.ResponseWriter, r *http.Request) {
			panic("test")
		})

		require.NotPanics(t, func() {
			router.ServeHTTP(w, req)
		})
	})
}
