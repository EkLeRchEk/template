package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTP      HTTPConfig
	Database  DatabaseConfig
	Positions PositionConfig
	LogLevel  string
}

type HTTPConfig struct {
	Addr            string
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	ConnectTimeout  time.Duration
	QueryTimeout    time.Duration
}

type PositionConfig struct {
	CheckInterval      time.Duration
	StaleAfter         time.Duration
	Workers            int
	QueueSize          int
	PushHTTPURL        string
	PushRequestTimeout time.Duration
}

func Load() (Config, error) {
	httpAddr, err := requiredEnv("HTTP_ADDR")
	if err != nil {
		return Config{}, err
	}

	logLevel, err := requiredEnv("LOG_LEVEL")
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := durationEnv("SHUTDOWN_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	databaseURL, err := requiredEnv("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}

	maxConns, err := int32Env("DATABASE_MAX_CONNS")
	if err != nil {
		return Config{}, err
	}

	minConns, err := int32Env("DATABASE_MIN_CONNS")
	if err != nil {
		return Config{}, err
	}

	maxConnLifetime, err := durationEnv("DATABASE_MAX_CONN_LIFETIME")
	if err != nil {
		return Config{}, err
	}

	connectTimeout, err := durationEnv("DATABASE_CONNECT_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	queryTimeout, err := durationEnv("DATABASE_QUERY_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	checkInterval, err := optionalDurationEnv(
		"POSITION_CHECK_INTERVAL", "30s",
	)
	if err != nil {
		return Config{}, err
	}

	staleAfter, err := optionalDurationEnv(
		"POSITION_STALE_AFTER", "60s",
	)
	if err != nil {
		return Config{}, err
	}

	workers, err := optionalIntEnv("POSITION_WORKERS", 4)
	if err != nil {
		return Config{}, err
	}

	queueSize, err := optionalIntEnv("POSITION_QUEUE_SIZE", 100)
	if err != nil {
		return Config{}, err
	}

	pushURL, err := requiredEnv("PUSH_HTTP_URL")
	if err != nil {
		return Config{}, err
	}

	pushTimeout, err := optionalDurationEnv(
		"PUSH_REQUEST_TIMEOUT", "1s",
	)
	if err != nil {
		return Config{}, err
	}

	if checkInterval <= 0 || staleAfter <= 0 || pushTimeout <= 0 {
		return Config{}, fmt.Errorf("position durations must be positive")
	}

	if workers <= 0 || queueSize <= 0 {
		return Config{}, fmt.Errorf("position workers and queue size must be positive")
	}

	return Config{
		HTTP: HTTPConfig{
			Addr:            httpAddr,
			ShutdownTimeout: shutdownTimeout,
		},
		Database: DatabaseConfig{
			URL:             databaseURL,
			MaxConns:        maxConns,
			MinConns:        minConns,
			MaxConnLifetime: maxConnLifetime,
			ConnectTimeout:  connectTimeout,
			QueryTimeout:    queryTimeout,
		},
		Positions: PositionConfig{
			CheckInterval:      checkInterval,
			StaleAfter:         staleAfter,
			Workers:            workers,
			QueueSize:          queueSize,
			PushHTTPURL:        pushURL,
			PushRequestTimeout: pushTimeout,
		},
		LogLevel: logLevel,
	}, nil
}

func requiredEnv(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf(
			"required environment variable %s is not set", name,
		)
	}

	return value, nil
}

func durationEnv(name string) (time.Duration, error) {
	value, err := requiredEnv(name)
	if err != nil {
		return 0, err
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}

	return duration, nil
}

func optionalDurationEnv(name, fallback string) (time.Duration, error) {
	value := os.Getenv(name)
	if value == "" {
		value = fallback
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}

	return duration, nil
}

func int32Env(name string) (int32, error) {
	value, err := requiredEnv(name)
	if err != nil {
		return 0, err
	}

	number, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}

	return int32(number), nil
}

func optionalIntEnv(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}

	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}

	return number, nil
}
