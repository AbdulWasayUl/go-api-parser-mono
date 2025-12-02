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

	chans := channels.New()

	wp := workpool.New(chans, 5)
	wp.Start(ctx)

	weatherSvc := weather.NewService(cfg)

	services := []scheduler.SchedulableService{weatherSvc}

	sch, err := scheduler.New()
	if err != nil {
		log.Fatalf("Failed to initialize scheduler: %v", err)
	}

	if err := sch.StartJob(ctx, client, chans, services); err != nil {
		log.Fatalf("Failed to start scheduler job: %v", err)
	}

	logger.Info("Executing immediate startup data fetch and store.")
	sch.RunImmediateJob(ctx, client, chans, services)

	<-quit
	logger.Info("Received interrupt signal. Shutting down gracefully...")

	sch.Cron.Stop()
	wp.Stop()

	logger.Info("Waiting for pending worker jobs to finish...")
	chans.WG.Wait()
	logger.Info("All worker jobs finished. Shutdown complete.")
}
