package scheduler

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/AbdulWasayUl/go-api-parser-mono/internal/channels"
	"github.com/AbdulWasayUl/go-api-parser-mono/models"
	"github.com/go-co-op/gocron"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeService implements SchedulableService for testing.
type fakeService struct {
	name      string
	callCount int
	mu        *sync.Mutex
	returnErr bool
}

func (f *fakeService) RunBatchJob(ctx context.Context, client interface{}, chans *channels.Channels) error {
	if f.mu != nil {
		f.mu.Lock()
		f.callCount++
		f.mu.Unlock()
	}

	// Simulate the service adding and completing work so the scheduler's wait doesn't block.
	if chans != nil && chans.WG != nil {
		chans.WG.Add(1)
		go func() {
			// simulate small work
			time.Sleep(5 * time.Millisecond)
			chans.WG.Done()
		}()
	}

	if f.returnErr {
		return fmt.Errorf("forced error")
	}
	return nil
}

func TestNew(t *testing.T) {
	s, err := New()
	require.NoError(t, err)
	require.NotNil(t, s)
	require.NotNil(t, s.Cron)
	require.NotNil(t, s.WG)
}

func TestRunImmediateJob_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		setupServices   func() []*fakeService
		expectCallCount int
	}{
		{
			name:            "no services",
			setupServices:   func() []*fakeService { return []*fakeService{} },
			expectCallCount: 0,
		},
		{
			name: "multiple services succeed",
			setupServices: func() []*fakeService {
				mu := &sync.Mutex{}
				return []*fakeService{
					{name: "s0", mu: mu},
					{name: "s1", mu: mu},
					{name: "s2", mu: mu},
				}
			},
			expectCallCount: 3,
		},
		{
			name: "one service errors but others run",
			setupServices: func() []*fakeService {
				mu := &sync.Mutex{}
				return []*fakeService{
					{name: "good1", mu: mu},
					{name: "bad", mu: mu, returnErr: true},
					{name: "good2", mu: mu},
				}
			},
			expectCallCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create services fresh for each subtest
			fakeServices := tt.setupServices()
			services := make([]SchedulableService, len(fakeServices))
			for i := range fakeServices {
				services[i] = fakeServices[i]
			}

			// prepare channels list matching services
			chanList := make([]*channels.Channels, len(services))
			for i := range chanList {
				chanList[i] = &channels.Channels{DataRequest: make(chan models.DataRequest, 1), WG: &sync.WaitGroup{}}
			}

			s := &Scheduler{Cron: gocron.NewScheduler(time.UTC), WG: &sync.WaitGroup{}}
			// RunImmediateJob should not panic and should complete (WGs handled by services)
			s.RunImmediateJob(context.Background(), nil, chanList, services)

			// Assert invocation counts for each service
			totalCalls := 0
			for _, fs := range fakeServices {
				totalCalls += fs.callCount
			}
			assert.Equal(t, tt.expectCallCount, totalCalls)
		})
	}
}

func TestStartJob_Basic(t *testing.T) {
	s := &Scheduler{Cron: gocron.NewScheduler(time.UTC), WG: &sync.WaitGroup{}}

	// StartJob schedules the job and starts the scheduler asynchronously. Passing nil client
	// and empty slices should be acceptable (job will reference nil but not execute immediately).
	err := s.StartJob(context.Background(), nil, []*channels.Channels{}, []SchedulableService{})
	require.NoError(t, err)

	// Stop the scheduler to avoid goroutine leaks in test runs.
	s.Cron.Stop()
}
