package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// InitTracing настраивает отправку трассировок в OTel Collector.
func InitTracing(ctx context.Context, serviceName string) (
	func(context.Context) error,
	error,
) {
	// Создаём OTLP HTTP exporter.
	// Адрес и протокол берутся из переменных OTEL_EXPORTER_OTLP_*.
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create OTLP exporter: %w", err)
	}

	// Указываем имя нашего сервиса.
	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create telemetry resource: %w", err)
	}

	// Настраиваем провайдер трассировок.
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Регистрируем его глобально.
	otel.SetTracerProvider(provider)

	// Настраиваем передачу trace context между сервисами.
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	// Возвращаем функцию корректного завершения.
	return provider.Shutdown, nil
}
