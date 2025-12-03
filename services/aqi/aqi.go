package aqi

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

type Service struct {
	Config *config.Config
	Client *api.Client
	DBName string
	mu     sync.Mutex
}

func NewService(cfg *config.Config) *Service {
	rlSettings := models.RateLimitSettings{
		MaxRequests: 40,
		PerDuration: time.Minute,
	}
	client := api.NewClient(rlSettings)

	return &Service{
		Config: cfg,
		Client: client,
		DBName: cfg.DBOpenAQ,
	}
}

func (s *Service) FetchData(ctx context.Context, id string) ([]byte, error) {
	countryCode := url.QueryEscape(id)
	url := fmt.Sprintf("%s/%s", s.Config.OpenAQAPIBaseURL, countryCode)
	headers := map[string]string{
		"X-API-Key": s.Config.OpenAQAPIKey,
	}
	return s.Client.Do(ctx, url, headers)
}

func (s *Service) ParseData(data []byte) (interface{}, error) {
	var resp AQIAPIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse AQI data: %w", err)
	}

	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	r := resp.Results[0]

	// map parameters
	params := make([]struct {
		Name  string `bson:"name"`
		Units string `bson:"units"`
	}, 0, len(r.Parameters))
	for _, p := range r.Parameters {
		params = append(params, struct {
			Name  string `bson:"name"`
			Units string `bson:"units"`
		}{
			Name:  p.Parameter,
			Units: p.Units,
		})
	}

	storeData := AQIData{
		CountryID:   r.Id,
		CountryName: r.Name,
		Parameters:  params,
		FetchedAt:   time.Now(),
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
		return fmt.Errorf("expected *mongo.Client, got %T", client)
	}

	aqiData, ok := data.(AQIData)
	if !ok {
		return fmt.Errorf("expected AQIData, got %T", data)
	}

	collection := client.Database(s.DBName).Collection(s.Config.CollectionDailyData)
	_, err := collection.InsertOne(ctx, aqiData)
	if err != nil {
		return fmt.Errorf("failed to insert AQI data: %w", err)
	}

	return nil
}

func (s *Service) RunBatchJob(ctx context.Context, client interface{}, chans *channels.Channels) error {

	logger.Info("[%s] Starting batch job...", s.DBName)

	client, ok := client.(*mongo.Client)
	if !ok {
		return fmt.Errorf("expected *mongo.Client, got %T", client)
	}

	params, err := db.GetFetchParams(ctx, client, s.DBName, s.Config.CollectionFetchParams)
	if err != nil {
		logger.Error("[%s] Failed to get fetch parameters: %v", s.DBName, err)
		return err
	}

	for _, param := range params {
		countryCode, ok := param["country_id"]
		if !ok {
			logger.Error("[%s] Invalid country_id in fetch parameters", s.DBName)
			continue
		}
		countryCodeFloat, ok := countryCode.(float64)
		if !ok {
			logger.Error("[%s] Invalid country_id type in fetch parameters", s.DBName)
			continue
		}
		countryIDStr := fmt.Sprintf("%d", int(countryCodeFloat))
		logger.Debug("countryIDStr: %s", countryIDStr)

		dataReq := models.DataRequest{
			ID:        countryIDStr,
			Service:   s.DBName,
			FetchFunc: s.FetchData,
			ParseFunc: s.ParseData,
			StoreFunc: func(ctx context.Context, data interface{}) error {
				return s.StoreData(ctx, client, data)
			},
		}
		chans.DataRequest <- dataReq
	}

	return nil
}
