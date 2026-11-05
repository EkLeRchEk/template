package middleware

import (
	"log/slog"
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"
)

// Logging записывает информацию о каждом HTTP-запросе.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Получаем статус HTTP-ответа.
		ww := chimiddleware.NewWrapResponseWriter(
			w,
			r.ProtoMajor,
		)

		next.ServeHTTP(ww, r)

		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}

		// Получаем Trace ID из контекста запроса.
		spanContext := trace.SpanContextFromContext(r.Context())

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", float64(time.Since(start).Microseconds()) / 1000,
		}

		if spanContext.IsValid() {
			attrs = append(attrs, "trace_id", spanContext.TraceID().String())
		}

		slog.InfoContext(
			r.Context(),
			"HTTP request completed",
			attrs...,
		)
	})
}
