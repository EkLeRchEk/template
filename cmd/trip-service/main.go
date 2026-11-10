package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/EkLeRchEk/template/internal/config"
	"github.com/EkLeRchEk/template/internal/container"
	httpmiddleware "github.com/EkLeRchEk/template/internal/http/middleware"
	"github.com/EkLeRchEk/template/internal/infra/postgres"
	"github.com/EkLeRchEk/template/internal/infra/telemetry"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Создаём экспорт логов в OpenTelemetry Collector.
	otelLogHandler, shutdownLogs, err := telemetry.InitLogs(
		ctx,
		"trip-service",
	)
	if err != nil {
		log.Fatalf("initialize OTEL logs: %v", err)
	}

	// Настраиваем JSON-логи и OTLP-экспорт.
	if err := telemetry.InitLogger(cfg.LogLevel, otelLogHandler); err != nil {
		log.Fatalf("initialize logger: %v", err)
	}

	// При завершении отправляем оставшиеся логи.
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		if err := shutdownLogs(shutdownCtx); err != nil {
			log.Printf("shutdown logs: %v", err)
		}
	}()

	// Инициализируем OpenTelemetry.
	shutdownTracing, err := telemetry.InitTracing(
		ctx,
		"trip-service",
	)
	if err != nil {
		log.Fatalf("initialize tracing: %v", err)
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		if err := shutdownTracing(shutdownCtx); err != nil {
			log.Printf("shutdown tracing: %v", err)
		}
	}()

	// Инициализируем OpenTelemetry Metrics.
	shutdownMetrics, err := telemetry.InitMetrics(
		ctx,
		"trip-service",
	)
	if err != nil {
		log.Fatalf("initialize metrics: %v", err)
	}

	// При остановке отправляем оставшиеся метрики.
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		if err := shutdownMetrics(shutdownCtx); err != nil {
			log.Printf("shutdown metrics: %v", err)
		}
	}()

	pool, err := postgres.NewPostgres(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	// Собираем зависимости новой архитектуры.
	app := container.New(
		pool,
		cfg.Database.QueryTimeout,
	)

	// Получаем HTTP-обработчик из контейнера.
	handler := app.HTTP.Handler()

	// Создаём middleware для сбора HTTP-метрик.
	metricsMiddleware, err := httpmiddleware.Metrics()
	if err != nil {
		log.Fatalf("initialize HTTP metrics: %v", err)
	}

	// Подключаем метрики ко всем HTTP-запросам.
	handler = metricsMiddleware(handler)

	// Логируем HTTP-запросы.
	handler = httpmiddleware.Logging(handler)

	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           otelhttp.NewHandler(handler, "trip-service-http"),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("HTTP server listening on %s", cfg.HTTP.Addr)

		if err := server.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server: %v", err)
		}
	}()

	<-ctx.Done()

	log.Println("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		cfg.HTTP.ShutdownTimeout,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown: %v", err)
	}

	log.Println("server stopped")
}
