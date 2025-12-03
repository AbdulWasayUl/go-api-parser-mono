package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/AbdulWasayUl/go-api-parser-mono/internal/channels"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/logger"
	"github.com/go-co-op/gocron"
	"go.mongodb.org/mongo-driver/mongo"
)

type SchedulableService interface {
	RunBatchJob(ctx context.Context, client interface{}, chans *channels.Channels) error
}

type Scheduler struct {
	Cron *gocron.Scheduler
	WG   *sync.WaitGroup
}

func New() (*Scheduler, error) {
	s := gocron.NewScheduler(time.UTC)
	return &Scheduler{
		Cron: s,
		WG:   &sync.WaitGroup{},
	}, nil
}

func (s *Scheduler) StartJob(ctx context.Context, client *mongo.Client, chanList []*channels.Channels, services []SchedulableService) error {

	_, err := s.Cron.Every(1).Day().At("07:30").Do(func() {
		s.runAllJobs(ctx, client, chanList, services)
	})
	if err != nil {
		return err
	}
	// _, err := s.Cron.Every(1).Minute().Do(func() {
	// 	s.runAllJobs(ctx, client, chans, services)
	// })
	// if err != nil {
	// 	logger.Error("Failed to schedule job: %v", err)
	// }

	s.Cron.StartAsync()
	return nil
}

func (s *Scheduler) runAllJobs(ctx context.Context, client *mongo.Client, chanList []*channels.Channels, services []SchedulableService) {
	startTime := time.Now()
	logger.Info("--- Fetch Job Started ---")
	defer func() {
		elapsed := time.Since(startTime)
		logger.Info("--- Fetch Job Finished --- (Total time: %v)", elapsed)
	}()

	for i, service := range services {
		currCh := chanList[i]
		err := service.RunBatchJob(ctx, client, currCh)
		if err != nil {
			logger.Error("Error running batch job for service: %v", err)
		}
	}

	logger.Info("Waiting for all submitted jobs to complete...")
	for _, ch := range chanList {
		ch.WG.Wait()
	}
	logger.Info("All jobs completed successfully.")
}

func (s *Scheduler) RunImmediateJob(ctx context.Context, client *mongo.Client, chanList []*channels.Channels, services []SchedulableService) {
	logger.Info("--- Immediate Fetch Job Started ---")
	defer logger.Info("--- Immediate Fetch Job Finished ---")

	s.runAllJobs(ctx, client, chanList, services)
}
