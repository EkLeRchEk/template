package middleware

import (
	"fmt"
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics создаёт middleware для измерения HTTP-запросов.
func Metrics() (func(http.Handler) http.Handler, error) {
	meter := otel.Meter("trip-service/http")

	// Счётчик HTTP-запросов.
	requests, err := meter.Int64Counter(
		"tripgo_http_requests",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create requests counter: %w", err)
	}

	// Гистограмма длительности запросов.
	duration, err := meter.Float64Histogram(
		"tripgo_http_request_duration",
		metric.WithDescription("HTTP request duration"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("create duration histogram: %w", err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Оборачиваем ResponseWriter, чтобы получить HTTP-статус.
			ww := chimiddleware.NewWrapResponseWriter(
				w,
				r.ProtoMajor,
			)

			next.ServeHTTP(ww, r)

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}

			// Атрибуты для группировки метрик.
			attrs := metric.WithAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.Int("http.response.status_code", status),
			)

			// Увеличиваем счётчик запросов.
			requests.Add(r.Context(), 1, attrs)

			// Записываем длительность в секундах.
			duration.Record(
				r.Context(),
				time.Since(start).Seconds(),
				attrs,
			)
		})
	}, nil
}
