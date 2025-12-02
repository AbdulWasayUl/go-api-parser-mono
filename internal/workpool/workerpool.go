package workpool

import (
	"context"
	"time"

	"github.com/AbdulWasayUl/go-api-parser-mono/internal/channels"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/logger"
	"github.com/AbdulWasayUl/go-api-parser-mono/models"
)

type WorkerPool struct {
	WorkerCount int
	Channels    *channels.Channels
}

func New(channels *channels.Channels, workerCount int) *WorkerPool {
	return &WorkerPool{
		WorkerCount: workerCount,
		Channels:    channels,
	}
}

func (wp *WorkerPool) Start(ctx context.Context) {
	for i := 0; i < wp.WorkerCount; i++ {
		go wp.worker(ctx, i)
	}
}

func (wp *WorkerPool) worker(ctx context.Context, id int) {
	logger.Info("Worker %d started.", id)
	for req := range wp.Channels.DataRequest {

		wp.Channels.WG.Add(1)

		func(req models.DataRequest) {
			defer wp.Channels.WG.Done()

			opCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			logger.Info("[%s] Worker %d processing request for ID: %s", req.Service, id, req.ID)

			// 1. Fetch Data
			data, err := req.FetchFunc(opCtx, req.ID)
			if err != nil {
				logger.Error("[%s] Worker %d failed to fetch data for %s: %v", req.Service, id, req.ID, err)
				return
			}

			// 2. Parse Data
			parsedData, err := req.ParseFunc(data)
			if err != nil {
				logger.Error("[%s] Worker %d failed to parse data for %s: %v", req.Service, id, req.ID, err)
				return
			}

			// 3. Store Data
			err = req.StoreFunc(opCtx, parsedData)
			if err != nil {
				logger.Error("[%s] Worker %d failed to store data for %s: %v", req.Service, id, req.ID, err)
				return
			}

			logger.Info("[%s] Worker %d successfully completed request for ID: %s", req.Service, id, req.ID)
		}(req)
	}

	logger.Info("Worker %d stopped.", id)
}

func (wp *WorkerPool) Stop() {
	close(wp.Channels.DataRequest)
}
