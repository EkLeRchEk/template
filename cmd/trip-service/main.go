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

	"github.com/EkLeRchEk/template/internal/config"
	"github.com/EkLeRchEk/template/internal/database"
	"github.com/EkLeRchEk/template/internal/httpserver"
	"github.com/EkLeRchEk/template/internal/trip"
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

	pool, err := database.NewPostgres(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	// Репозиторий для обычных запросов к БД.
	repo := trip.NewRepository(pool, cfg.Database.QueryTimeout)

	// Менеджер транзакций.
	txManager := database.NewTxManager(pool)

	// Сервис с бизнес-логикой.
	tripService := trip.NewService(
		repo,
		txManager,
		cfg.Database.QueryTimeout,
	)

	// HTTP-сервер со сгенерированными маршрутами OpenAPI.
	handler := httpserver.New(pool, tripService)

	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           handler.Handler(),
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
