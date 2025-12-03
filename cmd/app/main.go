package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/AbdulWasayUl/go-api-parser-mono/internal/channels"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/config"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/db"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/logger"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/scheduler"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/workpool"
	"github.com/AbdulWasayUl/go-api-parser-mono/services/aqi"
	"github.com/AbdulWasayUl/go-api-parser-mono/services/country"
	worldtime "github.com/AbdulWasayUl/go-api-parser-mono/services/time"
	"github.com/AbdulWasayUl/go-api-parser-mono/services/weather"
)

func main() {
	logger.Init()
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	client, err := db.ConnectMongoDB(ctx, cfg)
	if err != nil {
		logger.Error("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		if err := db.DisconnectMongoDB(ctx, client); err != nil {
			logger.Error("Error disconnecting MongoDB: %v", err)
		}
	}()

	if err := db.RunMigrations(ctx, client, cfg); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// chans := channels.New()

	// wp := workpool.New(chans, 30)
	// wp.Start(ctx)
	// Create 1 channels + 1 workerpool for each service
	chanList := make([]*channels.Channels, 0)
	wpList := make([]*workpool.WorkerPool, 0)

	// One per service
	for i := 0; i < 4; i++ {
		ch := channels.New()
		chanList = append(chanList, ch)

		wp := workpool.New(ch, 10)
		wp.Start(ctx)
		wpList = append(wpList, wp)
	}

	weatherSvc := weather.NewService(cfg)
	timeSvc := worldtime.NewService(cfg)
	countrySvc := country.NewService(cfg)
	aqiSvc := aqi.NewService(cfg)

	services := []scheduler.SchedulableService{weatherSvc, timeSvc, countrySvc, aqiSvc}
	// services := []scheduler.SchedulableService{aqiSvc}

	sch, err := scheduler.New()
	if err != nil {
		log.Fatalf("Failed to initialize scheduler: %v", err)
	}

	if err := sch.StartJob(ctx, client, chanList, services); err != nil {
		log.Fatalf("Failed to start scheduler job: %v", err)
	}

	logger.Info("Executing immediate startup data fetch and store.")
	sch.RunImmediateJob(ctx, client, chanList, services)

	<-quit
	logger.Info("Received interrupt signal. Shutting down gracefully...")

	sch.Cron.Stop()

	logger.Info("Waiting for pending worker jobs to finish...")

	// Stop all workerpools
	for _, wp := range wpList {
		wp.Stop()
	}

	// Wait for remaining work
	for _, ch := range chanList {
		ch.WG.Wait()
	}

	logger.Info("All worker jobs finished. Shutdown complete.")
}
