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
	positionworker "github.com/EkLeRchEk/template/internal/worker/positions"
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
		log.Printf("load config: %v", err)
		return
	}

	// Создаём экспорт логов в OpenTelemetry Collector.
	otelLogHandler, shutdownLogs, err := telemetry.InitLogs(
		ctx,
		"trip-service",
	)
	if err != nil {
		log.Printf("initialize OTEL logs: %v", err)
		return
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

	// Настраиваем JSON-логи и OTLP-экспорт.
	if err := telemetry.InitLogger(cfg.LogLevel, otelLogHandler); err != nil {
		log.Printf("initialize logger: %v", err)
		return
	}

	// Инициализируем OpenTelemetry Tracing.
	shutdownTracing, err := telemetry.InitTracing(
		ctx,
		"trip-service",
	)
	if err != nil {
		log.Printf("initialize tracing: %v", err)
		return
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
		log.Printf("initialize metrics: %v", err)
		return
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

	// Подключаемся к PostgreSQL.
	pool, err := postgres.NewPostgres(ctx, cfg.Database)
	if err != nil {
		log.Printf("connect to database: %v", err)
		return
	}
	defer pool.Close()

	// Собираем зависимости новой архитектуры.
	app, err := container.New(
		pool,
		cfg.Database.QueryTimeout,
		positionworker.Config{
			CheckInterval:      cfg.Positions.CheckInterval,
			StaleAfter:         cfg.Positions.StaleAfter,
			Workers:            cfg.Positions.Workers,
			QueueSize:          cfg.Positions.QueueSize,
			PushRequestTimeout: cfg.Positions.PushRequestTimeout,
		},
		cfg.Positions.PushHTTPURL,
	)
	if err != nil {
		log.Printf("initialize application: %v", err)
		return
	}

	// Получаем HTTP-обработчик.
	handler := app.HTTP.Handler()

	// Создаём middleware для сбора HTTP-метрик.
	metricsMiddleware, err := httpmiddleware.Metrics()
	if err != nil {
		log.Printf("initialize HTTP metrics: %v", err)
		return
	}

	// Worker Pool останавливается после завершения HTTP-сервера.
	app.Start(context.Background())

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

	// Запускаем HTTP-сервер.
	go func() {
		log.Printf("HTTP server listening on %s", cfg.HTTP.Addr)

		if err := server.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server: %v", err)
			stop()
		}
	}()

	// Ожидаем сигнал завершения.
	<-ctx.Done()

	log.Println("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		cfg.HTTP.ShutdownTimeout,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown: %v", err)
		_ = server.Close()
	}

	// Worker Pool останавливается после HTTP-сервера.
	if err := app.Stop(shutdownCtx); err != nil {
		log.Printf("position worker shutdown: %v", err)
	}

	log.Println("server stopped")
}
