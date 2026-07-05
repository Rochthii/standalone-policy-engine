package pectl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"time"
)

// APIError represents the RFC 7807 problem details structure.
type APIError struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
	InvalidParams []struct {
		Name   string `json:"name"`
		Reason string `json:"reason"`
	} `json:"invalid_params"`
}

func (e *APIError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%d] %s", e.Status, e.Title))
	if e.Detail != "" {
		sb.WriteString(fmt.Sprintf(": %s", e.Detail))
	}
	if len(e.InvalidParams) > 0 {
		sb.WriteString(" (params:")
		for _, p := range e.InvalidParams {
			sb.WriteString(fmt.Sprintf(" %s=%s", p.Name, p.Reason))
		}
		sb.WriteString(")")
	}
	return sb.String()
}

// Client is the HTTP client for the Control Plane API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	maxRetries int
}

// NewClient initializes a new Client with production-grade keep-alive and connection pooling.
func NewClient(baseURL, token string, timeout time.Duration) *Client {
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
	}

	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: tr,
		},
		maxRetries: 3,
	}
}

// Request performs an HTTP request with exponential backoff retry logic.
// Retries are only triggered on 5xx errors or network failures.
// 4xx errors are returned immediately without retry.
func (c *Client) Request(ctx context.Context, method, path string, payload interface{}, out interface{}) error {
	// Serialize payload once before retry loop to allow re-use.
	var rawBody []byte
	if payload != nil {
		var err error
		rawBody, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
	}

	url := fmt.Sprintf("%s%s", c.baseURL, path)

	var lastErr error
	backoff := 100 * time.Millisecond
	maxBackoff := 2 * time.Second

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff = time.Duration(math.Min(float64(backoff*2), float64(maxBackoff)))
			}
		}

		// Recreate reader each attempt so body can be re-sent on retry.
		var bodyReader io.Reader
		if rawBody != nil {
			bodyReader = bytes.NewReader(rawBody)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		if c.token != "" {
			// Never log the token value.
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request failed: %w", err)
			continue
		}

		// Read fully then close — do NOT defer inside loop to avoid goroutine leaks.
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", readErr)
			continue
		}

		// 2xx: success path.
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out != nil && len(respBody) > 0 {
				if err := json.Unmarshal(respBody, out); err != nil {
					return fmt.Errorf("failed to decode response: %w", err)
				}
			}
			return nil
		}

		// Try to parse as structured RFC 7807 APIError.
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Title != "" {
			apiErr.Status = resp.StatusCode
			lastErr = &apiErr
		} else {
			lastErr = fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
		}

		// Do not retry 4xx errors — only 5xx gets retried.
		if resp.StatusCode < 500 {
			return lastErr
		}
	}

	return fmt.Errorf("request failed after %d retries: %w", c.maxRetries, lastErr)
}
