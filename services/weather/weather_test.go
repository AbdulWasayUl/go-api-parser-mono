package weather

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/AbdulWasayUl/go-api-parser-mono/internal/channels"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/config"
	"github.com/AbdulWasayUl/go-api-parser-mono/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mongoContainer struct {
	tc.Container
	URI string
}

func setupMongoContainer(ctx context.Context) (*mongoContainer, error) {
	req := tc.ContainerRequest{
		Image:        "mongo:6",
		ExposedPorts: []string{"27017/tcp"},
		WaitingFor:   wait.ForLog("Waiting for connections"),
	}

	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := container.MappedPort(ctx, "27017")
	if err != nil {
		return nil, err
	}

	uri := fmt.Sprintf("mongodb://%s:%s", host, mappedPort.Port())

	return &mongoContainer{
		Container: container,
		URI:       uri,
	}, nil
}

func setupTestDB(ctx context.Context, uri, dbName string) (*mongo.Client, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return client, nil
}

func seedWeatherFetchParams(ctx context.Context, client *mongo.Client, dbName, collectionName string) error {
	collection := client.Database(dbName).Collection(collectionName)

	params := []interface{}{
		bson.M{"city": "London", "country": "United Kingdom", "lat": 51.5074, "lon": -0.1278},
		bson.M{"city": "Paris", "country": "France", "lat": 48.8566, "lon": 2.3522},
		bson.M{"city": "Berlin", "country": "Germany", "lat": 52.5200, "lon": 13.4050},
		bson.M{"city": "Madrid", "country": "Spain", "lat": 40.4168, "lon": -3.7038},
		bson.M{"city": "Rome", "country": "Italy", "lat": 41.9028, "lon": 12.4964},
	}

	_, err := collection.InsertMany(ctx, params)
	return err
}

func TestNewService(t *testing.T) {
	cfg := &config.Config{
		DBWeather:         "test_db",
		WeatherAPIBaseURL: "https://api.weatherapi.com/v1/current.json",
		WeatherAPIKey:     "test_key",
	}

	service := NewService(cfg)

	assert.NotNil(t, service)
	assert.Equal(t, cfg, service.Config)
	assert.NotNil(t, service.Client)
	assert.Equal(t, "test_db", service.DBName)
}

func TestParseData(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		validate    func(*testing.T, WeatherData)
	}{
		{
			name: "valid response with complete data",
			input: `{
				"location": {
					"name": "London",
					"country": "United Kingdom",
					"region": "City of London",
					"tz_id": "Europe/London"
				},
				"current": {
					"last_updated": "2024-01-15 10:30",
					"temp_c": 8.5,
					"temp_f": 47.3,
					"condition": {"text": "Partly cloudy"},
					"wind_kph": 12.6,
					"wind_mph": 7.8,
					"wind_dir": "NE",
					"wind_degree": 45,
					"pressure_mb": 1013.0,
					"pressure_in": 29.91,
					"precip_mm": 0.0,
					"precip_in": 0.0,
					"humidity": 72,
					"cloud": 50
				}
			}`,
			expectError: false,
			validate: func(t *testing.T, wd WeatherData) {
				assert.Equal(t, "London", wd.City)
				assert.Equal(t, "United Kingdom", wd.Country)
				assert.Equal(t, "City of London", wd.Region)
				assert.Equal(t, "Europe/London", wd.TzID)
				assert.Equal(t, 8.5, wd.TemperatureC)
				assert.Equal(t, 47.3, wd.TemperatureF)
				assert.Equal(t, "Partly cloudy", wd.Condition)
			},
		},
		{
			name:        "empty input",
			input:       ``,
			expectError: true,
		},
		{
			name:        "invalid json",
			input:       `{invalid json}`,
			expectError: true,
		},
	}

	cfg := &config.Config{}
	service := NewService(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := service.ParseData([]byte(tt.input))

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, data)

			weatherData, ok := data.(WeatherData)
			require.True(t, ok, "expected WeatherData type")

			if tt.validate != nil {
				tt.validate(t, weatherData)
			}

			// Verify FetchedAt is recent
			assert.WithinDuration(t, time.Now(), weatherData.FetchedAt, 5*time.Second)
		})
	}
}

func TestStoreData(t *testing.T) {
	ctx := context.Background()

	mongoC, err := setupMongoContainer(ctx)
	require.NoError(t, err)
	defer mongoC.Terminate(ctx)

	client, err := setupTestDB(ctx, mongoC.URI, "test_db")
	require.NoError(t, err)
	defer client.Disconnect(ctx)

	cfg := &config.Config{
		DBWeather:           "test_db",
		CollectionDailyData: "daily_data",
	}
	service := NewService(cfg)

	tests := []struct {
		name        string
		db          interface{}
		data        interface{}
		expectError bool
		errorMsg    string
		validate    func(*testing.T, *mongo.Client, *config.Config)
	}{
		{
			name: "successful insert",
			db:   client,
			data: WeatherData{
				City:         "London",
				Country:      "United Kingdom",
				Region:       "City of London",
				TzID:         "Europe/London",
				TemperatureF: 47.3,
				TemperatureC: 8.5,
				Condition:    "Partly cloudy",
				WindKPH:      12.6,
				WindMPH:      7.8,
				WindDir:      "NE",
				WindDegree:   45,
				PressureMB:   1013.0,
				PressureIN:   29.91,
				PrecipMM:     0.0,
				PrecipIN:     0.0,
				Humidity:     72,
				Cloud:        50,
				FetchedAt:    time.Now(),
			},
			expectError: false,
			validate: func(t *testing.T, client *mongo.Client, cfg *config.Config) {
				coll := client.Database(cfg.DBWeather).Collection(cfg.CollectionDailyData)
				count, err := coll.CountDocuments(ctx, bson.M{"city": "London"})
				require.NoError(t, err)
				assert.Equal(t, int64(1), count)
			},
		},
		{
			name:        "nil db client",
			db:          nil,
			data:        WeatherData{},
			expectError: true,
			errorMsg:    "mongo client is nil",
		},
		{
			name:        "invalid db type",
			db:          "not a mongo client",
			data:        WeatherData{},
			expectError: true,
			errorMsg:    "expected *mongo.Client",
		},
		{
			name:        "invalid data type",
			db:          client,
			data:        "invalid data",
			expectError: true,
			errorMsg:    "invalid data type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.StoreData(ctx, tt.db, tt.data)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				return
			}

			require.NoError(t, err)

			if tt.validate != nil {
				tt.validate(t, client, cfg)
			}
		})
	}
}

func TestRunBatchJob(t *testing.T) {
	ctx := context.Background()

	mongoC, err := setupMongoContainer(ctx)
	require.NoError(t, err)
	defer mongoC.Terminate(ctx)

	client, err := setupTestDB(ctx, mongoC.URI, "test_db")
	require.NoError(t, err)
	defer client.Disconnect(ctx)

	cfg := &config.Config{
		DBWeather:             "test_db",
		CollectionFetchParams: "fetch_params",
		CollectionDailyData:   "daily_data",
	}
	service := NewService(cfg)

	tests := []struct {
		name          string
		setup         func() error
		expectError   bool
		expectedCount int
	}{
		{
			name: "successful batch job with params",
			setup: func() error {
				return seedWeatherFetchParams(ctx, client, cfg.DBWeather, cfg.CollectionFetchParams)
			},
			expectError:   false,
			expectedCount: 5,
		},
		{
			name: "batch job with no params",
			setup: func() error {
				coll := client.Database(cfg.DBWeather).Collection(cfg.CollectionFetchParams)
				coll.DeleteMany(ctx, bson.M{})
				return nil
			},
			expectError:   false,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				err := tt.setup()
				require.NoError(t, err)
			}

			// Create channels
			chans := &channels.Channels{
				DataRequest: make(chan models.DataRequest, 100),
			}

			var wg sync.WaitGroup
			var requestCount int
			var mu sync.Mutex

			// Goroutine to count requests
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range chans.DataRequest {
					mu.Lock()
					requestCount++
					mu.Unlock()
				}
			}()

			// Run batch job
			err := service.RunBatchJob(ctx, client, chans)
			close(chans.DataRequest)
			wg.Wait()

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, requestCount)
		})
	}
}

func TestParseDataEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		validate    func(*testing.T, WeatherData)
	}{
		{
			name: "very low temperature (cold)",
			input: `{
				"location": {
					"name": "Oymyakon",
					"country": "Russia",
					"region": "Siberia",
					"tz_id": "Asia/Yakutsk"
				},
				"current": {
					"temp_c": -71.2,
					"temp_f": -96.2,
					"condition": {"text": "Arctic cold"},
					"wind_kph": 5.0,
					"wind_mph": 3.1,
					"wind_dir": "N",
					"wind_degree": 0,
					"pressure_mb": 1040.0,
					"pressure_in": 30.71,
					"precip_mm": 0.0,
					"precip_in": 0.0,
					"humidity": 85,
					"cloud": 50
				}
			}`,
			expectError: false,
			validate: func(t *testing.T, wd WeatherData) {
				assert.Equal(t, -71.2, wd.TemperatureC)
				assert.Equal(t, -96.2, wd.TemperatureF)
				assert.Equal(t, "Oymyakon", wd.City)
			},
		},
		{
			name: "high wind speed",
			input: `{
				"location": {
					"name": "Wellington",
					"country": "New Zealand",
					"region": "Wellington",
					"tz_id": "Pacific/Auckland"
				},
				"current": {
					"temp_c": 15.0,
					"temp_f": 59.0,
					"condition": {"text": "Windy"},
					"wind_kph": 85.0,
					"wind_mph": 52.8,
					"wind_dir": "NW",
					"wind_degree": 315,
					"pressure_mb": 995.0,
					"pressure_in": 29.38,
					"precip_mm": 10.0,
					"precip_in": 0.39,
					"humidity": 75,
					"cloud": 80
				}
			}`,
			expectError: false,
			validate: func(t *testing.T, wd WeatherData) {
				assert.Equal(t, 85.0, wd.WindKPH)
				assert.Equal(t, 52.8, wd.WindMPH)
			},
		},
		{
			name: "heavy precipitation",
			input: `{
				"location": {
					"name": "Cherrapunji",
					"country": "India",
					"region": "Meghalaya",
					"tz_id": "Asia/Kolkata"
				},
				"current": {
					"temp_c": 18.5,
					"temp_f": 65.3,
					"condition": {"text": "Heavy rain"},
					"wind_kph": 25.0,
					"wind_mph": 15.5,
					"wind_dir": "SW",
					"wind_degree": 225,
					"pressure_mb": 1005.0,
					"pressure_in": 29.68,
					"precip_mm": 250.0,
					"precip_in": 9.84,
					"humidity": 98,
					"cloud": 100
				}
			}`,
			expectError: false,
			validate: func(t *testing.T, wd WeatherData) {
				assert.Equal(t, 250.0, wd.PrecipMM)
				assert.Equal(t, 9.84, wd.PrecipIN)
				assert.Equal(t, 98, wd.Humidity)
			},
		},
		{
			name: "all wind directions",
			input: `{
				"location": {
					"name": "Test",
					"country": "Test",
					"region": "Test",
					"tz_id": "UTC"
				},
				"current": {
					"temp_c": 20.0,
					"temp_f": 68.0,
					"condition": {"text": "Test"},
					"wind_kph": 10.0,
					"wind_mph": 6.2,
					"wind_dir": "ESE",
					"wind_degree": 112,
					"pressure_mb": 1013.25,
					"pressure_in": 29.92,
					"precip_mm": 0.0,
					"precip_in": 0.0,
					"humidity": 50,
					"cloud": 25
				}
			}`,
			expectError: false,
			validate: func(t *testing.T, wd WeatherData) {
				assert.Equal(t, "ESE", wd.WindDir)
				assert.Equal(t, 112, wd.WindDegree)
			},
		},
	}

	cfg := &config.Config{}
	service := NewService(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := service.ParseData([]byte(tt.input))

			require.NoError(t, err)
			require.NotNil(t, data)

			weatherData, ok := data.(WeatherData)
			require.True(t, ok, "expected WeatherData type")

			if tt.validate != nil {
				tt.validate(t, weatherData)
			}
		})
	}
}
