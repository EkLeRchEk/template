package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

var ErrTransport = errors.New("push service transport error")

type HTTPStatusError struct {
	StatusCode int
	Body       string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf(
		"push service returned HTTP %d: %s",
		e.StatusCode,
		e.Body,
	)
}

type Client struct {
	baseURL string
	client  *http.Client
}

type pushRequest struct {
	RequestID   string            `json:"request_id"`
	RecipientID string            `json:"recipient_id"`
	Kind        string            `json:"kind"`
	Data        map[string]string `json:"data,omitempty"`
}

func New(baseURL string, timeout time.Duration) (*Client, error) {
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse push service URL: %w", err)
	}

	if (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Host == "" {
		return nil, fmt.Errorf("invalid push service URL")
	}

	if timeout <= 0 {
		return nil, fmt.Errorf("push timeout must be positive")
	}

	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// SendPositionRequest отправляет команду REQUEST_POSITION.
func (c *Client) SendPositionRequest(
	ctx context.Context,
	driverID string,
	tripID string,
) error {
	ctx, span := otel.Tracer("trip-service/push").Start(
		ctx,
		"PushService.SendPositionRequest",
	)
	defer span.End()

	payload := pushRequest{
		RequestID:   uuid.NewString(),
		RecipientID: driverID,
		Kind:        "REQUEST_POSITION",
		Data: map[string]string{
			"trip_id": tripID,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "marshal push request failed")
		return fmt.Errorf("marshal push request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/v1/push",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("create push request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "push transport failed")
		return fmt.Errorf("%w: %v", ErrTransport, err)
	}
	defer resp.Body.Close()

	responseBody, readErr := io.ReadAll(
		io.LimitReader(resp.Body, 4096),
	)
	if readErr != nil {
		span.RecordError(readErr)
		return fmt.Errorf("read push response: %w", readErr)
	}

	if resp.StatusCode != http.StatusAccepted {
		err := &HTTPStatusError{
			StatusCode: resp.StatusCode,
			Body:       string(responseBody),
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "push service rejected request")

		return err
	}

	return nil
}

// Close закрывает простаивающие HTTP-соединения.
func (c *Client) Close() {
	c.client.CloseIdleConnections()
}
