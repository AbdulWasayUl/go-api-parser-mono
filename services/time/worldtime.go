package worldtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/AbdulWasayUl/go-api-parser-mono/internal/api"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/channels"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/config"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/db"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/logger"
	"github.com/AbdulWasayUl/go-api-parser-mono/models"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	baseURL              = "https://worldtimeapi.org/api/timezone"
	fetchParamCollection = "fetch_params"
	dailyDataCollection  = "daily_data"
)

type Service struct {
	Config *config.Config
	Client *api.Client
	DBName string
	mu     sync.Mutex
}

func NewService(cfg *config.Config) *Service {
	rlSettings := models.RateLimitSettings{
		MaxRequests: 20,
		PerDuration: time.Minute,
	}
	client := api.NewClient(rlSettings)

	return &Service{
		Config: cfg,
		Client: client,
		DBName: cfg.DBWorldTime,
	}
}

func (s *Service) FetchData(ctx context.Context, id string) ([]byte, error) {
	timezone := url.QueryEscape(id)
	url := fmt.Sprintf("%s/%s", baseURL, timezone)
	return s.Client.Do(ctx, url, nil)
}

func (s *Service) ParseData(data []byte) (interface{}, error) {
	var resp WorldTimeAPIResponse
	err := json.Unmarshal(data, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse world time data: %w", err)
	}
	storeData := WorldTimeData{
		Timezone:     resp.Timezone,
		UTCOffset:    resp.UTCOffset,
		CurrentTime:  resp.Datetime,
		DayOfWeek:    resp.DayOfWeek,
		WeekNumber:   resp.WeekNumber,
		UTCDatetime:  resp.UTCDatetime,
		IsDST:        resp.DST,
		Abbreviation: resp.Abbreviation,
		FetchedAt:    time.Now(),
	}

	return storeData, nil
}

func (s *Service) StoreData(ctx context.Context, db interface{}, data interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if db == nil {
		return fmt.Errorf("mongo client is nil")
	}
	client, ok := db.(*mongo.Client)
	if !ok {
		return fmt.Errorf("expected *mongo.Client, got %T", db)
	}

	weatherData, ok := data.(WorldTimeData)
	if !ok {
		return fmt.Errorf("invalid data type for storing weather data")
	}

	coll := client.Database(s.DBName).Collection(dailyDataCollection)
	_, err := coll.InsertOne(ctx, weatherData)

	return err
}

func (s *Service) RunBatchJob(ctx context.Context, client interface{}, chans *channels.Channels) error {

	logger.Info("[%s] Starting batch job...", s.DBName)

	client, ok := client.(*mongo.Client)
	if !ok {
		return fmt.Errorf("expected *mongo.Client, got %T", client)
	}

	params, err := db.GetFetchParams(ctx, client, s.DBName, fetchParamCollection)
	if err != nil {
		logger.Error("[%s] Failed to get fetch parameters: %v", s.DBName, err)
		return err
	}

	for _, param := range params {
		timezone, ok := param["timezone"].(string)
		if !ok {
			logger.Error("[%s] Invalid city parameter: %v", s.DBName, param["timezone"])
			continue
		}
		dataReq := models.DataRequest{
			ID:        timezone,
			Service:   s.DBName,
			FetchFunc: s.FetchData,
			ParseFunc: s.ParseData,
			StoreFunc: func(ctx context.Context, data interface{}) error {
				return s.StoreData(ctx, client, data)
			},
			// StoreFunc: s.StoreData,
		}
		chans.DataRequest <- dataReq
	}

	logger.Info("[%s] Submitted %d requests to the worker pool.", s.DBName, len(params))
	return nil
}
