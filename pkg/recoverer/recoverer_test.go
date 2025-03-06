package recoverer

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	t.Run("should return internal server error on panic", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		router := chi.NewRouter()
		router.Use(Handler)
		router.Get("/", func(w http.ResponseWriter, r *http.Request) {
			panic("test")
		})
		router.ServeHTTP(w, req)

		body, err := io.ReadAll(w.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusInternalServerError, w.Code)
		require.Equal(t, "\"test\"\n", string(body))
	})
}
