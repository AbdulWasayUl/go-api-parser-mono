package api_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AbdulWasayUl/go-api-parser-mono/internal/api"
	"github.com/AbdulWasayUl/go-api-parser-mono/models"

	"github.com/joho/godotenv"
)

// load env from project root
func init() {
	root, err := findProjectRoot()
	if err != nil {
		log.Fatalf("failed to find project root: %v", err)
	}
	envFile := filepath.Join(root, ".env")
	if _, err := os.Stat(envFile); err == nil {
		if err := godotenv.Load(envFile); err != nil {
			log.Fatalf("failed to load .env: %v", err)
		}
	}
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if fileExists(filepath.Join(dir, "go.mod")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Test real APIs (except flaky WorldTime)
func TestClient_Do_Real_APIs(t *testing.T) {
	weatherURL := os.Getenv("WEATHER_API_BASE_URL")
	weatherKey := os.Getenv("WEATHER_API_KEY")
	openaqURL := os.Getenv("OPENAQ_API_BASE_URL")
	openaqKey := os.Getenv("OPENAQ_API_KEY")
	rcURL := os.Getenv("RESTCOUNTRIES_API_BASE_URL")

	if weatherURL == "" || openaqURL == "" || rcURL == "" {
		t.Fatal("missing required env variables")
	}

	tests := []struct {
		name       string
		url        string
		headers    map[string]string
		wantErr    bool
		minBodyLen int
	}{
		{
			name:       "Weather API - London",
			url:        weatherURL + "?key=" + weatherKey + "&q=London",
			minBodyLen: 50,
		},
		{
			name:       "OpenAQ API - Code 1",
			url:        openaqURL + "/1",
			headers:    map[string]string{"X-API-Key": openaqKey},
			minBodyLen: 50,
		},
		{
			name:       "RestCountries - Pakistan",
			url:        rcURL + "/PK",
			minBodyLen: 20,
		},
		{
			name:    "Context Timeout",
			url:     weatherURL + "?key=" + weatherKey + "&q=London",
			wantErr: true,
		},
	}

	client := api.NewClient(models.RateLimitSettings{MaxRequests: 5, PerDuration: time.Second})

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.wantErr {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 1*time.Millisecond)
				defer cancel()
			}

			body, err := client.Do(ctx, tt.url, tt.headers)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(body) < tt.minBodyLen {
				t.Fatalf("response too small (%d bytes)", len(body))
			}
			t.Logf("OK: %s => %d bytes", tt.name, len(body))
		})
	}
}

// Test retry, backoff, and 4xx/5xx handling using httptest server
func TestClient_Do_Retries(t *testing.T) {
	serverHits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHits++
		switch serverHits {
		case 1, 2:
			w.WriteHeader(500)
			fmt.Fprintln(w, "server error")
		case 3:
			w.WriteHeader(200)
			fmt.Fprintln(w, "success after retries")
		}
	}))
	defer ts.Close()

	client := api.NewClient(models.RateLimitSettings{MaxRequests: 2, PerDuration: time.Second})

	ctx := context.Background()
	body, err := client.Do(ctx, ts.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) == "" {
		t.Fatalf("empty body returned")
	}
	if serverHits != 3 {
		t.Fatalf("expected 3 attempts, got %d", serverHits)
	}

	// Test fatal 4xx
	ts4xx := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		fmt.Fprintln(w, "not found")
	}))
	defer ts4xx.Close()

	_, err = client.Do(ctx, ts4xx.URL, nil)
	if err == nil {
		t.Fatalf("expected error for 4xx, got none")
	}
}

// Test rate limiting
func TestClient_Do_RateLimit(t *testing.T) {
	client := api.NewClient(models.RateLimitSettings{MaxRequests: 2, PerDuration: 1 * time.Second})
	ctx := context.Background()

	start := time.Now()
	for i := 0; i < 4; i++ {
		_, _ = client.Do(ctx, "https://httpbin.org/get", nil)
	}
	elapsed := time.Since(start)
	if elapsed < 1*time.Second {
		t.Fatalf("rate limiting not enforced")
	}
}
