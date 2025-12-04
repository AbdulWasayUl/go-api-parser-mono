package workpool_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AbdulWasayUl/go-api-parser-mono/internal/channels"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/workpool"
	"github.com/AbdulWasayUl/go-api-parser-mono/models"
)

func TestWorkerPool_New(t *testing.T) {
	ch := &channels.Channels{
		DataRequest: make(chan models.DataRequest, 10),
		WG:          &sync.WaitGroup{},
	}
	workerCount := 3

	wp := workpool.New(ch, workerCount)

	if wp == nil {
		t.Fatal("Expected WorkerPool to be created")
	}
	if wp.WorkerCount != workerCount {
		t.Errorf("Expected WorkerCount %d, got %d", workerCount, wp.WorkerCount)
	}
	if wp.Channels != ch {
		t.Error("Expected Channels to match")
	}
}

func TestWorkerPool_SingleJob(t *testing.T) {
	ch := channels.New()
	wp := workpool.New(ch, 1)

	completed := make(chan bool, 3)

	req := models.DataRequest{
		Service: "test",
		ID:      "123",
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) {
			completed <- true
			return []byte("data"), nil
		},
		ParseFunc: func(data []byte) (interface{}, error) {
			completed <- true
			return "parsed", nil
		},
		StoreFunc: func(ctx context.Context, d interface{}) error {
			completed <- true
			return nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wp.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	ch.DataRequest <- req

	done := make(chan struct{})
	go func() {
		ch.WG.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for jobs to complete")
	}

	wp.Stop()

	// Check that all three steps completed
	count := 0
	for i := 0; i < 3; i++ {
		select {
		case <-completed:
			count++
		case <-time.After(100 * time.Millisecond):
			i = 3 // Exit loop
		}
	}

	if count != 3 {
		t.Errorf("Expected all 3 steps to complete, got %d", count)
	}
}

func TestWorkerPool_MultipleJobs(t *testing.T) {
	ch := channels.New()
	wp := workpool.New(ch, 2)

	var completed int32
	mu := &sync.Mutex{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wp.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	numJobs := 5
	for i := 0; i < numJobs; i++ {
		req := models.DataRequest{
			ID: "job",
			FetchFunc: func(ctx context.Context, id string) ([]byte, error) {
				return []byte("data"), nil
			},
			ParseFunc: func(data []byte) (interface{}, error) {
				return "result", nil
			},
			StoreFunc: func(ctx context.Context, d interface{}) error {
				mu.Lock()
				completed++
				mu.Unlock()
				return nil
			},
		}
		ch.DataRequest <- req
	}

	done := make(chan struct{})
	go func() {
		ch.WG.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(3 * time.Second):
		mu.Lock()
		c := completed
		mu.Unlock()
		t.Fatalf("Timeout waiting for jobs: completed %d/%d", c, numJobs)
	}

	wp.Stop()

	mu.Lock()
	c := completed
	mu.Unlock()

	if int(c) != numJobs {
		t.Errorf("Expected %d jobs completed, got %d", numJobs, c)
	}
}

func TestWorkerPool_FetchError(t *testing.T) {
	ch := channels.New()
	wp := workpool.New(ch, 1)

	var parseCalled, storeCalled atomic.Bool

	req := models.DataRequest{
		ID: "test",
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) {
			return nil, errors.New("fetch failed")
		},
		ParseFunc: func(data []byte) (interface{}, error) {
			parseCalled.Store(true)
			return nil, nil
		},
		StoreFunc: func(ctx context.Context, d interface{}) error {
			storeCalled.Store(true)
			return nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wp.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	ch.DataRequest <- req

	done := make(chan struct{})
	go func() {
		ch.WG.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for error handling")
	}

	wp.Stop()

	if parseCalled.Load() || storeCalled.Load() {
		t.Error("Parse and Store should not be called after Fetch error")
	}
}

func TestWorkerPool_ParseError(t *testing.T) {
	ch := channels.New()
	wp := workpool.New(ch, 1)

	var storeCalled atomic.Bool

	req := models.DataRequest{
		ID: "test",
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) {
			return []byte("data"), nil
		},
		ParseFunc: func(data []byte) (interface{}, error) {
			return nil, errors.New("parse failed")
		},
		StoreFunc: func(ctx context.Context, d interface{}) error {
			storeCalled.Store(true)
			return nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wp.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	ch.DataRequest <- req

	done := make(chan struct{})
	go func() {
		ch.WG.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for error handling")
	}

	wp.Stop()

	if storeCalled.Load() {
		t.Error("Store should not be called after Parse error")
	}
}

func TestWorkerPool_StoreError(t *testing.T) {
	ch := channels.New()
	wp := workpool.New(ch, 1)

	req := models.DataRequest{
		ID: "test",
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) {
			return []byte("data"), nil
		},
		ParseFunc: func(data []byte) (interface{}, error) {
			return "parsed", nil
		},
		StoreFunc: func(ctx context.Context, d interface{}) error {
			return errors.New("store failed")
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wp.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	ch.DataRequest <- req

	done := make(chan struct{})
	go func() {
		ch.WG.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for store error")
	}

	wp.Stop()

	// Should not panic or hang
}

func TestWorkerPool_Stop(t *testing.T) {
	ch := channels.New()
	wp := workpool.New(ch, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wp.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	wp.Stop()

	// Should not panic - channel is closed
}

func TestWorkerPool_NoJobs(t *testing.T) {
	ch := channels.New()
	wp := workpool.New(ch, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wp.Start(ctx)
	wp.Stop()

	// WG.Wait should return immediately with no jobs
	done := make(chan struct{})
	go func() {
		ch.WG.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("WG.Wait timed out with no jobs")
	}
}
