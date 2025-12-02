package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/AbdulWasayUl/go-api-parser-mono/internal/logger"
	"github.com/AbdulWasayUl/go-api-parser-mono/models"
)

type Client struct {
	httpClient *http.Client
	rateLimit  models.RateLimitSettings
	limiter    *time.Ticker
}

func NewClient(rl models.RateLimitSettings) *Client {
	interval := rl.PerDuration / time.Duration(rl.MaxRequests)

	ticker := time.NewTicker(interval)

	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		rateLimit:  rl,
		limiter:    ticker,
	}
}

func (c *Client) Do(ctx context.Context, url string, headers map[string]string) ([]byte, error) {

	<-c.limiter.C

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		logger.Info("Making request to %s (attempt %d)", url, i+1)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			logger.Error("HTTP request failed (attempt %d): %v", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return io.ReadAll(resp.Body)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			logger.Error("API returned 429 Too Many Requests (attempt %d)", i+1)
			time.Sleep(5 * time.Second)
			continue
		}

		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			logger.Error("API returned status code %d (attempt %d). Body: %s", resp.StatusCode, i+1, string(body))
			return nil, errors.New("API returned non-OK status: " + resp.Status)
		}
	}

	return nil, errors.New("failed to fetch data after max retries")
}
