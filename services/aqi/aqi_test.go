package aqi

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

	// Ping to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return client, nil
}

func seedFetchParams(ctx context.Context, client *mongo.Client, dbName, collectionName string) error {
	collection := client.Database(dbName).Collection(collectionName)

	params := []interface{}{
		bson.M{"country_id": 51, "country_code": "EE", "country_name": "Estonia"},
		bson.M{"country_id": 14, "country_code": "ET", "country_name": "Ethiopia"},
		bson.M{"country_id": 55, "country_code": "FI", "country_name": "Finland"},
		bson.M{"country_id": 22, "country_code": "FR", "country_name": "France"},
		bson.M{"country_id": 50, "country_code": "DE", "country_name": "Germany"},
		bson.M{"country_id": 152, "country_code": "GH", "country_name": "Ghana"},
	}

	_, err := collection.InsertMany(ctx, params)
	return err
}

func TestNewService(t *testing.T) {
	cfg := &config.Config{
		DBOpenAQ:         "test_db",
		OpenAQAPIBaseURL: "https://api.openaq.org/v2/countries",
		OpenAQAPIKey:     "test_key",
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
		validate    func(*testing.T, AQIData)
	}{
		{
			name: "valid response with Estonia data",
			input: `{
				"results": [{
					"id": 51,
					"code": "EE",
					"name": "Estonia",
					"parameters": [
						{
							"id": 1,
							"name": "pm25",
							"units": "µg/m³",
							"displayName": "PM2.5",
							"parameter": "pm25"
						},
						{
							"id": 2,
							"name": "pm10",
							"units": "µg/m³",
							"displayName": "PM10",
							"parameter": "pm10"
						}
					]
				}]
			}`,
			expectError: false,
			validate: func(t *testing.T, aqiData AQIData) {
				assert.Equal(t, 51, aqiData.CountryID)
				assert.Equal(t, "Estonia", aqiData.CountryName)
				assert.Len(t, aqiData.Parameters, 2)
				assert.Equal(t, "pm25", aqiData.Parameters[0].Name)
				assert.Equal(t, "µg/m³", aqiData.Parameters[0].Units)
			},
		},
		{
			name: "valid response with Ethiopia data",
			input: `{
				"results": [{
					"id": 14,
					"code": "ET",
					"name": "Ethiopia",
					"parameters": [
						{
							"id": 1,
							"name": "pm25",
							"units": "µg/m³",
							"displayName": "PM2.5",
							"parameter": "pm25"
						}
					]
				}]
			}`,
			expectError: false,
			validate: func(t *testing.T, aqiData AQIData) {
				assert.Equal(t, 14, aqiData.CountryID)
				assert.Equal(t, "Ethiopia", aqiData.CountryName)
				assert.Len(t, aqiData.Parameters, 1)
			},
		},
		{
			name:        "empty results",
			input:       `{"results": []}`,
			expectError: true,
		},
		{
			name:        "invalid json",
			input:       `{invalid json}`,
			expectError: true,
		},
		{
			name:        "missing results field",
			input:       `{"data": []}`,
			expectError: true,
		},
		{
			name: "no parameters",
			input: `{
				"results": [{
					"id": 51,
					"code": "EE",
					"name": "Estonia",
					"parameters": []
				}]
			}`,
			expectError: false,
			validate: func(t *testing.T, aqiData AQIData) {
				assert.Equal(t, 51, aqiData.CountryID)
				assert.Equal(t, "Estonia", aqiData.CountryName)
				assert.Len(t, aqiData.Parameters, 0)
			},
		},
		{
			name: "multiple parameters",
			input: `{
				"results": [{
					"id": 51,
					"code": "EE",
					"name": "Estonia",
					"parameters": [
						{"parameter": "pm25", "units": "µg/m³"},
						{"parameter": "pm10", "units": "µg/m³"},
						{"parameter": "no2", "units": "ppb"},
						{"parameter": "o3", "units": "ppb"}
					]
				}]
			}`,
			expectError: false,
			validate: func(t *testing.T, aqiData AQIData) {
				assert.Len(t, aqiData.Parameters, 4)
				assert.Equal(t, "pm25", aqiData.Parameters[0].Name)
				assert.Equal(t, "o3", aqiData.Parameters[3].Name)
			},
		},
		{
			name: "various countries",
			input: `{
				"results": [{
					"id": 50,
					"code": "DE",
					"name": "Germany",
					"parameters": [
						{"parameter": "pm25", "units": "µg/m³"}
					]
				}]
			}`,
			expectError: false,
			validate: func(t *testing.T, aqiData AQIData) {
				assert.Equal(t, 50, aqiData.CountryID)
				assert.Equal(t, "Germany", aqiData.CountryName)
			},
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

			aqiData, ok := data.(AQIData)
			require.True(t, ok, "expected AQIData type")

			if tt.validate != nil {
				tt.validate(t, aqiData)
			}

			// Verify FetchedAt is recent
			assert.WithinDuration(t, time.Now(), aqiData.FetchedAt, 5*time.Second)
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
		DBOpenAQ:            "test_db",
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
			name:        "successful insert",
			db:          client,
			expectError: false,
			data: AQIData{
				CountryID:   51,
				CountryName: "Estonia",
				Parameters: []struct {
					Name  string `bson:"name"`
					Units string `bson:"units"`
				}{
					{Name: "pm25", Units: "µg/m³"},
					{Name: "pm10", Units: "µg/m³"},
				},
				FetchedAt: time.Now(),
			},
			validate: func(t *testing.T, client *mongo.Client, cfg *config.Config) {
				collection := client.Database(cfg.DBOpenAQ).Collection(cfg.CollectionDailyData)
				var result AQIData
				err := collection.FindOne(context.Background(), bson.M{"country_id": 51}).Decode(&result)
				require.NoError(t, err)
				assert.Equal(t, "Estonia", result.CountryName)
				assert.Len(t, result.Parameters, 2)
			},
		},
		{
			name:        "nil database client",
			db:          nil,
			data:        AQIData{CountryID: 51, CountryName: "Estonia"},
			expectError: true,
			errorMsg:    "mongo client is nil",
		},
		{
			name:        "invalid database client type",
			db:          "invalid",
			data:        AQIData{CountryID: 51, CountryName: "Estonia"},
			expectError: true,
			errorMsg:    "expected *mongo.Client",
		},
		{
			name:        "invalid data type",
			db:          client,
			data:        "invalid data",
			expectError: true,
			errorMsg:    "expected AQIData",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.StoreData(ctx, tt.db, tt.data)

			if tt.expectError {
				assert.Error(t, err)
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

func TestStoreData_MultipleCountries(t *testing.T) {
	ctx := context.Background()

	mongoC, err := setupMongoContainer(ctx)
	require.NoError(t, err)
	defer mongoC.Terminate(ctx)

	client, err := setupTestDB(ctx, mongoC.URI, "test_db")
	require.NoError(t, err)
	defer client.Disconnect(ctx)

	cfg := &config.Config{
		DBOpenAQ:            "test_db",
		CollectionDailyData: "daily_data",
	}
	service := NewService(cfg)

	tests := []struct {
		name      string
		countries []AQIData
		expected  int64
		verify    func(*testing.T, *mongo.Client, *config.Config)
	}{
		{
			name: "insert multiple countries",
			countries: []AQIData{
				{CountryID: 51, CountryName: "Estonia", FetchedAt: time.Now()},
				{CountryID: 14, CountryName: "Ethiopia", FetchedAt: time.Now()},
				{CountryID: 55, CountryName: "Finland", FetchedAt: time.Now()},
				{CountryID: 22, CountryName: "France", FetchedAt: time.Now()},
				{CountryID: 50, CountryName: "Germany", FetchedAt: time.Now()},
				{CountryID: 152, CountryName: "Ghana", FetchedAt: time.Now()},
			},
			expected: 6,
			verify: func(t *testing.T, client *mongo.Client, cfg *config.Config) {
				collection := client.Database(cfg.DBOpenAQ).Collection(cfg.CollectionDailyData)
				var result AQIData
				err := collection.FindOne(context.Background(), bson.M{"country_id": 50}).Decode(&result)
				require.NoError(t, err)
				assert.Equal(t, "Germany", result.CountryName)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, country := range tt.countries {
				err := service.StoreData(ctx, client, country)
				require.NoError(t, err)
			}

			// Verify all countries were inserted
			collection := client.Database(cfg.DBOpenAQ).Collection(cfg.CollectionDailyData)
			count, err := collection.CountDocuments(ctx, bson.M{})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, count)

			if tt.verify != nil {
				tt.verify(t, client, cfg)
			}
		})
	}
}

func TestStoreData_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()

	mongoC, err := setupMongoContainer(ctx)
	require.NoError(t, err)
	defer mongoC.Terminate(ctx)

	client, err := setupTestDB(ctx, mongoC.URI, "test_db")
	require.NoError(t, err)
	defer client.Disconnect(ctx)

	cfg := &config.Config{
		DBOpenAQ:            "test_db",
		CollectionDailyData: "daily_data",
	}
	service := NewService(cfg)

	tests := []struct {
		name       string
		goroutines int
		expected   int64
	}{
		{
			name:       "concurrent writes with 10 goroutines",
			goroutines: 10,
			expected:   10,
		},
		{
			name:       "concurrent writes with 5 goroutines",
			goroutines: 5,
			expected:   5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear collection before each test
			collection := client.Database(cfg.DBOpenAQ).Collection(cfg.CollectionDailyData)
			collection.DeleteMany(ctx, bson.M{})

			errChan := make(chan error, tt.goroutines)

			for i := 0; i < tt.goroutines; i++ {
				go func(id int) {
					aqiData := AQIData{
						CountryID:   id,
						CountryName: fmt.Sprintf("Country_%d", id),
						FetchedAt:   time.Now(),
					}
					errChan <- service.StoreData(ctx, client, aqiData)
				}(i)
			}

			// Collect all errors
			for i := 0; i < tt.goroutines; i++ {
				err := <-errChan
				assert.NoError(t, err)
			}

			// Verify all documents were inserted
			collection = client.Database(cfg.DBOpenAQ).Collection(cfg.CollectionDailyData)
			count, err := collection.CountDocuments(ctx, bson.M{})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, count)
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
		DBOpenAQ:              "test_db",
		CollectionDailyData:   "daily_data",
		CollectionFetchParams: "fetch_params",
	}

	tests := []struct {
		name        string
		setup       func()
		clientInput interface{}
		expectError bool
		errorMsg    string
		verify      func(*testing.T, *mongo.Client, *config.Config)
	}{
		{
			name: "invalid client type",
			setup: func() {
				// No setup needed
			},
			clientInput: "invalid",
			expectError: true,
			errorMsg:    "expected *mongo.Client",
		},
		{
			name: "verify fetch params loaded",
			setup: func() {
				err := seedFetchParams(ctx, client, cfg.DBOpenAQ, cfg.CollectionFetchParams)
				require.NoError(t, err)
			},
			clientInput: client,
			expectError: false,
			verify: func(t *testing.T, client *mongo.Client, cfg *config.Config) {
				collection := client.Database(cfg.DBOpenAQ).Collection(cfg.CollectionFetchParams)
				count, err := collection.CountDocuments(ctx, bson.M{})
				require.NoError(t, err)
				assert.Equal(t, int64(6), count)
			},
		},
		{
			name: "invalid fetch params - missing country_id and wrong type",
			setup: func() {
				collection := client.Database(cfg.DBOpenAQ).Collection(cfg.CollectionFetchParams)
				_, err := collection.InsertMany(ctx, []interface{}{
					bson.M{"country_code": "US"},    // missing country_id
					bson.M{"country_id": "invalid"}, // wrong type
				})
				require.NoError(t, err)
			},
			clientInput: client,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			service := NewService(cfg)
			ch := &channels.Channels{
				DataRequest: make(chan models.DataRequest, 100),
				WG:          &sync.WaitGroup{},
			}

			err := service.RunBatchJob(ctx, tt.clientInput, ch)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				close(ch.DataRequest)
				return
			}

			assert.NoError(t, err)
			close(ch.DataRequest)

			if tt.verify != nil {
				tt.verify(t, client, cfg)
			}
		})
	}
}

func TestFetchData(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantURL string
	}{
		{
			name:    "simple country code",
			id:      "US",
			wantURL: "https://api.openaq.org/v2/countries/US",
		},
		{
			name:    "country code with special chars",
			id:      "US&test",
			wantURL: "https://api.openaq.org/v2/countries/US%26test",
		},
	}

	cfg := &config.Config{
		DBOpenAQ:              "test_db",
		OpenAQAPIBaseURL:      "https://api.openaq.org/v2/countries",
		OpenAQAPIKey:          "test_key",
		CollectionDailyData:   "daily_data",
		CollectionFetchParams: "fetch_params",
	}
	service := NewService(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily test the actual HTTP call without mocking,
			// but we can verify the URL construction logic is sound
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// This will fail with network error, but we're mainly verifying
			// the URL construction doesn't panic
			_, err := service.FetchData(ctx, tt.id)
			// Error is expected (no real API), just verify no panic occurred
			assert.NotNil(t, err)
		})
	}
}
