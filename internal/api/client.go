package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/AbdulWasayUl/go-api-parser-mono/internal/logger"
	"github.com/AbdulWasayUl/go-api-parser-mono/models"
)

type Client struct {
	httpClient *http.Client
	rateLimit  models.RateLimitSettings
	tokens     chan struct{} // token bucket
}

func NewClient(rl models.RateLimitSettings) *Client {
	bucket := make(chan struct{}, rl.MaxRequests)

	// fill bucket initially
	for i := 0; i < rl.MaxRequests; i++ {
		bucket <- struct{}{}
	}

	// refill bucket at interval
	go func() {
		ticker := time.NewTicker(rl.PerDuration / time.Duration(rl.MaxRequests))
		defer ticker.Stop()

		for range ticker.C {
			select {
			case bucket <- struct{}{}:
			default:
				// bucket full
			}
		}
	}()

	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		rateLimit:  rl,
		tokens:     bucket,
	}
}

func (c *Client) Do(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	// Acquire a rate-limit token
	select {
	case <-c.tokens:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	const maxRetries = 5

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		for k, v := range headers {
			req.Header.Set(k, v)
		}

		logger.Info("Making request to %s (attempt %d)", url, i+1)
		resp, err := c.httpClient.Do(req)

		if err != nil {
			// network error, retry with backoff
			logger.Error("HTTP request failed (attempt %d): %v", i+1, err)

			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			backoff(i)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 200 {
			return body, nil
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			logger.Error("Server returned %d → retry (attempt %d)", resp.StatusCode, i+1)
			backoff(i)
			continue
		}

		// 4xx other than 429 = fatal
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, body)
	}

	return nil, errors.New("max retries exceeded")
}

func backoff(i int) {
	// exponential backoff with jitter
	base := time.Duration(1<<i) * time.Second
	jitter := time.Duration(rand.Intn(500)) * time.Millisecond
	time.Sleep(base + jitter)
}

// package api

// import (
// 	"context"
// 	"errors"
// 	"fmt"
// 	"io"
// 	"net/http"
// 	"time"

// 	"github.com/AbdulWasayUl/go-api-parser-mono/internal/logger"
// 	"github.com/AbdulWasayUl/go-api-parser-mono/models"
// )

// type Client struct {
// 	httpClient *http.Client
// 	rateLimit  models.RateLimitSettings
// 	limiter    *time.Ticker
// }

// func NewClient(rl models.RateLimitSettings) *Client {
// 	interval := rl.PerDuration / time.Duration(rl.MaxRequests)

// 	ticker := time.NewTicker(interval)

// 	return &Client{
// 		httpClient: &http.Client{Timeout: 20 * time.Second},
// 		rateLimit:  rl,
// 		limiter:    ticker,
// 	}
// }

// func (c *Client) Do(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
// 	<-c.limiter.C

// 	const maxRetries = 10
// 	for i := 0; i < maxRetries; i++ {
// 		// Create a fresh request for each attempt
// 		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to create request: %w", err)
// 		}

// 		for key, value := range headers {
// 			req.Header.Set(key, value)
// 		}

// 		logger.Info("Making request to %s (attempt %d)", url, i+1)
// 		resp, err := c.httpClient.Do(req)
// 		if err != nil {
// 			logger.Error("HTTP request failed (attempt %d): %v", i+1, err)

// 			// Check if context is cancelled - if so, don't retry
// 			if ctx.Err() != nil {
// 				return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
// 			}

// 			time.Sleep(time.Duration(2*i) * time.Second)
// 			continue
// 		}
// 		defer resp.Body.Close()

// 		if resp.StatusCode == http.StatusOK {
// 			return io.ReadAll(resp.Body)
// 		}

// 		if resp.StatusCode == http.StatusTooManyRequests {
// 			logger.Error("API returned 429 Too Many Requests (attempt %d)", i+1)
// 			time.Sleep(5 * time.Second)
// 			continue
// 		}

// 		if resp.StatusCode >= 400 {
// 			body, _ := io.ReadAll(resp.Body)
// 			logger.Error("API returned status code %d (attempt %d). Body: %s", resp.StatusCode, i+1, string(body))
// 			return nil, errors.New("API returned non-OK status: " + resp.Status)
// 		}
// 	}

// 	return nil, errors.New("failed to fetch data after max retries")
// }
