package telemetry

import (
	"context"
	"log/slog"
)

// multiHandler отправляет логи сразу нескольким обработчикам.
type multiHandler struct {
	handlers []slog.Handler
}

// NewMultiHandler объединяет несколько обработчиков логов.
func NewMultiHandler(handlers ...slog.Handler) slog.Handler {
	return &multiHandler{handlers: handlers}
}

// Enabled проверяет, нужен ли лог хотя бы одному обработчику.
func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle передаёт лог всем подходящим обработчикам.
func (h *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var result error

	for _, handler := range h.handlers {
		if !handler.Enabled(ctx, record.Level) {
			continue
		}

		if err := handler.Handle(ctx, record.Clone()); err != nil {
			if result == nil {
				result = err
			}
		}
	}

	return result
}

// WithAttrs добавляет атрибуты во все обработчики.
func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))

	for _, handler := range h.handlers {
		handlers = append(handlers, handler.WithAttrs(attrs))
	}

	return NewMultiHandler(handlers...)
}

// WithGroup добавляет группу атрибутов.
func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))

	for _, handler := range h.handlers {
		handlers = append(handlers, handler.WithGroup(name))
	}

	return NewMultiHandler(handlers...)
}
