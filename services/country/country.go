package country

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/AbdulWasayUl/go-api-parser-mono/internal/api"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/config"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/db"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/logger"
	"github.com/AbdulWasayUl/go-api-parser-mono/models"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	baseURL              = "https://restcountries.com/v3.1/alpha"
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
		DBName: cfg.DBRestCountries,
	}
}

func (s *Service) FetchData(ctx context.Context, id string) ([]byte, error) {
	countryCode := url.QueryEscape(id)
	url := fmt.Sprintf("%s/%s", baseURL, countryCode)
	return s.Client.Do(ctx, url, nil)
}

func (s *Service) ParseData(data []byte) (interface{}, error) {
	var resp RestCountriesAPIResponse

	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse country data: %w", err)
	}

	if len(resp) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	r := resp[0]

	capital := ""
	if len(r.Capital) > 0 {
		capital = r.Capital[0]
	}

	var currencyName, currencySym string
	for _, c := range r.Currencies {
		currencyName = c.Name
		currencySym = c.Symbol
		break
	}

	storeData := CountryData{
		CountryCode:  r.CCA2,
		OfficialName: r.Name.Official,
		CommonName:   r.Name.Common,
		Independent:  r.Independent,
		UnMember:     r.UnMember,
		Capital:      capital,
		Region:       r.Region,
		Subregion:    r.Subregion,
		Currency:     currencyName,
		CurrencySym:  currencySym,
		Population:   r.Population,
		Area:         r.Area,
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

	countryData, ok := data.(CountryData)
	if !ok {
		return fmt.Errorf("expected CountryData, got %T", data)
	}

	coll := client.Database(s.DBName).Collection(dailyDataCollection)
	_, err := coll.InsertOne(ctx, countryData)
	if err != nil {
		return fmt.Errorf("failed to store country data: %w", err)
	}

	return nil
}

func (s *Service) RunBatchJob(ctx context.Context, client interface{}, out chan<- models.DataRequest) error {

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
		countryCode, ok := param["country_code"].(string)
		if !ok {
			logger.Error("[%s] Invalid country_code in fetch parameters", s.DBName)
			continue
		}

		dataReq := models.DataRequest{
			ID:        countryCode,
			Service:   s.DBName,
			FetchFunc: s.FetchData,
			ParseFunc: s.ParseData,
			StoreFunc: func(ctx context.Context, data interface{}) error {
				return s.StoreData(ctx, client, data)
			},
		}
		out <- dataReq
	}

	return nil
}
