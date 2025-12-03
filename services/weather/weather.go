package weather

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
		MaxRequests: 30,
		PerDuration: time.Minute,
	}
	client := api.NewClient(rlSettings)

	return &Service{
		Config: cfg,
		Client: client,
		DBName: cfg.DBWeather,
	}
}

func (s *Service) FetchData(ctx context.Context, id string) ([]byte, error) {
	city := url.QueryEscape(id)
	url := fmt.Sprintf("%s?key=%s&q=%s", s.Config.WeatherAPIBaseURL, s.Config.WeatherAPIKey, city)
	return s.Client.Do(ctx, url, nil)
}

func (s *Service) ParseData(data []byte) (interface{}, error) {
	var resp WeatherAPIResponse
	err := json.Unmarshal(data, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse weather data: %w", err)
	}

	storeData := WeatherData{
		City:         resp.Location.Name,
		Country:      resp.Location.Country,
		Region:       resp.Location.Region,
		TzID:         resp.Location.TzID,
		TemperatureF: resp.Current.TempF,
		TemperatureC: resp.Current.TempC,
		Condition:    resp.Current.Condition.Text,
		WindKPH:      resp.Current.WindKPH,
		WindMPH:      resp.Current.WindMPH,
		WindDir:      resp.Current.WindDir,
		WindDegree:   resp.Current.WindDegree,
		PressureMB:   resp.Current.PressureMB,
		PressureIN:   resp.Current.PressureIN,
		PrecipMM:     resp.Current.PrecipMM,
		PrecipIN:     resp.Current.PrecipIN,
		Humidity:     resp.Current.Humidity,
		Cloud:        resp.Current.Cloud,
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

	weatherData, ok := data.(WeatherData)
	if !ok {
		return fmt.Errorf("invalid data type for storing weather data")
	}

	coll := client.Database(s.DBName).Collection(s.Config.CollectionDailyData)
	_, err := coll.InsertOne(ctx, weatherData)

	return err
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
		city, ok := param["city"].(string)
		if !ok {
			logger.Error("[%s] Invalid city parameter: %v", s.DBName, param["city"])
			continue
		}
		dataReq := models.DataRequest{
			ID:        city,
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
