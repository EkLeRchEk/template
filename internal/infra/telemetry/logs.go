package telemetry

import (
	"context"
	"fmt"

	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// InitLogs настраивает экспорт логов в OpenTelemetry Collector.
func InitLogs(
	ctx context.Context,
	serviceName string,
) (slog.Handler, func(context.Context) error, error) {

	// Создаём OTLP HTTP экспортёр логов.
	exporter, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("create logs exporter: %w", err)
	}

	// Указываем имя сервиса.
	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return nil, nil, fmt.Errorf("create logs resource: %w", err)
	}

	// Отправляем логи пакетами.
	processor := sdklog.NewBatchProcessor(exporter)

	// Создаём провайдер логов.
	provider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(processor),
	)

	// Создаём мост между slog и OpenTelemetry.
	handler := otelslog.NewHandler(
		"trip-service",
		otelslog.WithLoggerProvider(provider),
	)

	return handler, provider.Shutdown, nil
}
