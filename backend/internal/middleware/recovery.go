package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/eva-bharat/media-sequencer/backend/internal/utils"
)

// Recovery catches any panics during request execution and responds with a safe 500 error.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered in HTTP handler",
					"error", rec,
					"stack", string(debug.Stack()),
					"path", r.URL.Path,
				)
				utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected server error occurred")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
