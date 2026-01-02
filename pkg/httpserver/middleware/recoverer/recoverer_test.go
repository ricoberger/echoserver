package recoverer

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	t.Run("should return internal server error on panic", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		mux := http.NewServeMux()
		mux.Handle("/", Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test")
		})))
		mux.ServeHTTP(w, req)

		body, err := io.ReadAll(w.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusInternalServerError, w.Code)
		require.Contains(t, string(body), "test")
	})
}
