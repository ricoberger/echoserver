package recoverer

import (
	"fmt"
	"log/slog"
	"net/http"
)

func Handler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil && err != http.ErrAbortHandler {
				slog.ErrorContext(r.Context(), "Recover panic.", slog.Any("error", err))
				http.Error(w, fmt.Sprintf("%#v", err), http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}
