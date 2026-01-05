package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Gobusters/ectologger"
)

const (
	// DefaultTimeout is the default request timeout
	DefaultTimeout = 30 * time.Second

	// MaxResponseSize is the maximum response body size (10MB)
	MaxResponseSize = 10 * 1024 * 1024

	// MaxRequestSize is the maximum request body size (5MB)
	MaxRequestSize = 5 * 1024 * 1024
)

// Client wraps the HTTP client with logging and size limits
type Client struct {
	client *http.Client
	logger ectologger.Logger
}

// Config holds HTTP client configuration
type Config struct {
	Timeout            time.Duration
	MaxIdleConns       int
	IdleConnTimeout    time.Duration
	DisableCompression bool
	DisableKeepAlives  bool
}

// DefaultConfig returns default HTTP client configuration
func DefaultConfig() Config {
	return Config{
		Timeout:            DefaultTimeout,
		MaxIdleConns:       100,
		IdleConnTimeout:    90 * time.Second,
		DisableCompression: false,
		DisableKeepAlives:  false,
	}
}

// NewClient creates a new HTTP client
func NewClient(cfg Config, logger ectologger.Logger) *Client {
	transport := &http.Transport{
		MaxIdleConns:       cfg.MaxIdleConns,
		IdleConnTimeout:    cfg.IdleConnTimeout,
		DisableCompression: cfg.DisableCompression,
		DisableKeepAlives:  cfg.DisableKeepAlives,
	}

	return &Client{
		client: &http.Client{
			Transport: transport,
			Timeout:   cfg.Timeout,
		},
		logger: logger,
	}
}

// Response represents an HTTP response
type Response struct {
	StatusCode    int               `json:"status_code"`
	Headers       map[string]string `json:"headers"`
	Body          []byte            `json:"-"`
	BodyJSON      any               `json:"body,omitempty"`
	ContentType   string            `json:"content_type"`
	ContentLength int64             `json:"content_length"`
	Duration      time.Duration     `json:"duration_ms"`
}

// Do executes an HTTP request and returns the response
func (c *Client) Do(ctx context.Context, req *http.Request) (*Response, error) {
	start := time.Now()

	// Execute request
	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil {
		c.logger.WithContext(ctx).WithError(err).Errorf("HTTP request failed: %s %s", req.Method, req.URL.String())
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	duration := time.Since(start)

	// Check response size
	if resp.ContentLength > MaxResponseSize {
		return nil, fmt.Errorf("response too large: %d bytes (max %d)", resp.ContentLength, MaxResponseSize)
	}

	// Read response body with size limit
	limitedReader := io.LimitReader(resp.Body, MaxResponseSize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if len(body) > MaxResponseSize {
		return nil, fmt.Errorf("response body too large: %d bytes (max %d)", len(body), MaxResponseSize)
	}

	// Extract headers
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	response := &Response{
		StatusCode:    resp.StatusCode,
		Headers:       headers,
		Body:          body,
		ContentType:   resp.Header.Get("Content-Type"),
		ContentLength: int64(len(body)),
		Duration:      duration,
	}

	c.logger.WithContext(ctx).Debugf("HTTP %s %s -> %d (%s)",
		req.Method, req.URL.String(), resp.StatusCode, duration)

	return response, nil
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, url string, headers map[string]string) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return c.Do(ctx, req)
}

// SetTimeout sets a custom timeout for the client
func (c *Client) SetTimeout(timeout time.Duration) {
	c.client.Timeout = timeout
}
