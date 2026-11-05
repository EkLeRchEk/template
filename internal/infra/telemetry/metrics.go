package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// InitMetrics настраивает экспорт метрик в OTel Collector.
func InitMetrics(
	ctx context.Context,
	serviceName string,
) (func(context.Context) error, error) {

	// Создаём экспортёр OTLP HTTP.
	exporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create metrics exporter: %w", err)
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
		return nil, fmt.Errorf("create metrics resource: %w", err)
	}

	// Отправляем метрики каждые 10 секунд.
	reader := sdkmetric.NewPeriodicReader(
		exporter,
		sdkmetric.WithInterval(10*time.Second),
	)

	// Создаём провайдер метрик.
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)

	// Регистрируем глобальный провайдер.
	otel.SetMeterProvider(provider)

	return provider.Shutdown, nil
}
