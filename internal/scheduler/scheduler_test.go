package scheduler_test

// import (
// 	"context"
// 	"errors"
// 	"testing"
// 	"time"

// 	"github.com/AbdulWasayUl/go-api-parser-mono/internal/channels"
// 	"github.com/AbdulWasayUl/go-api-parser-mono/internal/logger"
// 	"github.com/AbdulWasayUl/go-api-parser-mono/internal/scheduler"
// )

// // mock service implementing SchedulableService
// type mockService struct {
// 	called bool
// 	err    error
// }

// func (m *mockService) RunBatchJob(ctx context.Context, client interface{}, chans *channels.Channels) error {
// 	m.called = true
// 	// simulate sending a job to channel
// 	chans.DataRequest <- struct{}{} // empty struct for test
// 	chans.WG.Add(1)
// 	chans.WG.Done()
// 	return m.err
// }

// func TestScheduler_RunImmediateJob_Table(t *testing.T) {
// 	logger.Init() // optional: enable logging to stdout during tests

// 	tests := []struct {
// 		name          string
// 		serviceErr    error
// 		expectedError bool
// 	}{
// 		{"SingleService_NoError", nil, false},
// 		{"SingleService_WithError", errors.New("job failed"), false}, // error is logged, not returned
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			ctx := context.Background()
// 			sched, err := scheduler.New()
// 			if err != nil {
// 				t.Fatalf("failed to create scheduler: %v", err)
// 			}

// 			mock := &mockService{err: tt.serviceErr}
// 			ch := channels.New()
// 			chanList := []*channels.Channels{ch}
// 			services := []scheduler.SchedulableService{mock}

// 			sched.RunImmediateJob(ctx, nil, chanList, services)

// 			if !mock.called {
// 				t.Errorf("expected service RunBatchJob to be called")
// 			}

// 			// Ensure WaitGroup completes
// 			doneCh := make(chan struct{})
// 			go func() {
// 				ch.WG.Wait()
// 				close(doneCh)
// 			}()

// 			select {
// 			case <-doneCh:
// 				// ok
// 			case <-time.After(1 * time.Second):
// 				t.Errorf("WaitGroup did not complete in time")
// 			}
// 		})
// 	}
// }

// func TestScheduler_StartJob(t *testing.T) {
// 	ctx := context.Background()
// 	sched, err := scheduler.New()
// 	if err != nil {
// 		t.Fatalf("failed to create scheduler: %v", err)
// 	}

// 	mock := &mockService{}
// 	ch := channels.New()
// 	chanList := []*channels.Channels{ch}
// 	services := []scheduler.SchedulableService{mock}

// 	err = sched.StartJob(ctx, nil, chanList, services)
// 	if err != nil {
// 		t.Fatalf("StartJob returned error: %v", err)
// 	}

// 	// Wait briefly to let cron job (simulated) run once
// 	time.Sleep(100 * time.Millisecond)

// 	if !mock.called {
// 		t.Errorf("expected service RunBatchJob to be called by cron job")
// 	}

// 	// Stop cron to avoid leaking goroutines
// 	sched.Cron.Stop()
// }
