package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// InitLogger настраивает JSON-логирование и экспорт в OpenTelemetry.
func InitLogger(level string, otelHandler slog.Handler) error {
	var logLevel slog.Level

	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		return fmt.Errorf("unknown log level: %s", level)
	}

	// JSON-логи остаются в терминале.
	jsonHandler := slog.NewJSONHandler(
		os.Stdout,
		&slog.HandlerOptions{
			Level: logLevel,
		},
	)

	// Отправляем сообщения в оба направления.
	handler := NewMultiHandler(
		jsonHandler,
		otelHandler,
	)

	// Общий фильтр уровня для обоих обработчиков.
	logger := slog.New(&minLevelHandler{
		Handler: handler,
		level:   logLevel,
	})
	slog.SetDefault(logger)

	return nil
}

type minLevelHandler struct {
	slog.Handler
	level slog.Level
}

func (h *minLevelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level && h.Handler.Enabled(ctx, level)
}

func (h *minLevelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &minLevelHandler{
		Handler: h.Handler.WithAttrs(attrs),
		level:   h.level,
	}
}

func (h *minLevelHandler) WithGroup(name string) slog.Handler {
	return &minLevelHandler{
		Handler: h.Handler.WithGroup(name),
		level:   h.level,
	}
}
