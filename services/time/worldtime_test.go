package worldtime

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

func seedTimezoneFetchParams(ctx context.Context, client *mongo.Client, dbName, collectionName string) error {
	collection := client.Database(dbName).Collection(collectionName)

	params := []interface{}{
		bson.M{"country_code": "US", "timezone": "America/New_York"},
		bson.M{"country_code": "UK", "timezone": "Europe/London"},
		bson.M{"country_code": "JP", "timezone": "Asia/Tokyo"},
		bson.M{"country_code": "AU", "timezone": "Australia/Sydney"},
		bson.M{"country_code": "IN", "timezone": "Asia/Kolkata"},
	}

	_, err := collection.InsertMany(ctx, params)
	return err
}

func TestNewService(t *testing.T) {
	cfg := &config.Config{
		DBWorldTime:         "test_db",
		WorldTimeAPIBaseURL: "http://worldtimeapi.org/api/timezone",
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
		validate    func(*testing.T, WorldTimeData)
	}{
		{
			name: "valid response with all fields",
			input: `{
				"abbreviation": "EST",
				"client_ip": "203.0.113.1",
				"client_ip_uri": "https://en.wikipedia.org/wiki/203.0.113.0/24",
				"datetime": "2023-12-04T10:30:00.123456-05:00",
				"day_of_week": 1,
				"dst": false,
				"dst_from": null,
				"dst_offset": 0,
				"dst_until": null,
				"raw_offset": -18000,
				"timezone": "America/New_York",
				"unixtime": 1701694200,
				"utc_datetime": "2023-12-04T15:30:00.123456+00:00",
				"utc_offset": "-05:00",
				"week_number": 49
			}`,
			expectError: false,
			validate: func(t *testing.T, data WorldTimeData) {
				assert.Equal(t, "America/New_York", data.Timezone)
				assert.Equal(t, "-05:00", data.UTCOffset)
				assert.Equal(t, 1, data.DayOfWeek)
				assert.Equal(t, 49, data.WeekNumber)
				assert.False(t, data.IsDST)
				assert.Equal(t, "EST", data.Abbreviation)
			},
		},
		{
			name: "valid response with DST enabled",
			input: `{
				"abbreviation": "EDT",
				"client_ip": "203.0.113.1",
				"datetime": "2023-06-15T10:30:00.123456-04:00",
				"day_of_week": 4,
				"dst": true,
				"dst_from": "2023-03-12T07:00:00+00:00",
				"dst_offset": 3600,
				"dst_until": "2023-11-05T06:00:00+00:00",
				"raw_offset": -18000,
				"timezone": "America/New_York",
				"unixtime": 1701694200,
				"utc_datetime": "2023-06-15T14:30:00.123456+00:00",
				"utc_offset": "-04:00",
				"week_number": 24
			}`,
			expectError: false,
			validate: func(t *testing.T, data WorldTimeData) {
				assert.Equal(t, "America/New_York", data.Timezone)
				assert.True(t, data.IsDST)
				assert.Equal(t, "EDT", data.Abbreviation)
				assert.Equal(t, "-04:00", data.UTCOffset)
			},
		},
		{
			name: "valid response for London timezone",
			input: `{
				"abbreviation": "GMT",
				"client_ip": "203.0.113.1",
				"datetime": "2023-12-04T15:30:00.123456+00:00",
				"day_of_week": 1,
				"dst": false,
				"raw_offset": 0,
				"timezone": "Europe/London",
				"unixtime": 1701694200,
				"utc_datetime": "2023-12-04T15:30:00.123456+00:00",
				"utc_offset": "+00:00",
				"week_number": 49
			}`,
			expectError: false,
			validate: func(t *testing.T, data WorldTimeData) {
				assert.Equal(t, "Europe/London", data.Timezone)
				assert.Equal(t, "+00:00", data.UTCOffset)
				assert.Equal(t, "GMT", data.Abbreviation)
			},
		},
		{
			name: "valid response for Tokyo timezone (positive offset)",
			input: `{
				"abbreviation": "JST",
				"client_ip": "203.0.113.1",
				"datetime": "2023-12-05T00:30:00.123456+09:00",
				"day_of_week": 2,
				"dst": false,
				"raw_offset": 32400,
				"timezone": "Asia/Tokyo",
				"unixtime": 1701694200,
				"utc_datetime": "2023-12-04T15:30:00.123456+00:00",
				"utc_offset": "+09:00",
				"week_number": 49
			}`,
			expectError: false,
			validate: func(t *testing.T, data WorldTimeData) {
				assert.Equal(t, "Asia/Tokyo", data.Timezone)
				assert.Equal(t, "+09:00", data.UTCOffset)
				assert.Equal(t, "JST", data.Abbreviation)
				assert.Equal(t, 2, data.DayOfWeek)
			},
		},
		{
			name: "valid response for Sydney timezone",
			input: `{
				"abbreviation": "AEDT",
				"client_ip": "203.0.113.1",
				"datetime": "2023-12-05T02:30:00.123456+11:00",
				"day_of_week": 2,
				"dst": true,
				"raw_offset": 36000,
				"timezone": "Australia/Sydney",
				"unixtime": 1701694200,
				"utc_datetime": "2023-12-04T15:30:00.123456+00:00",
				"utc_offset": "+11:00",
				"week_number": 49
			}`,
			expectError: false,
			validate: func(t *testing.T, data WorldTimeData) {
				assert.Equal(t, "Australia/Sydney", data.Timezone)
				assert.Equal(t, "+11:00", data.UTCOffset)
				assert.True(t, data.IsDST)
			},
		},
		{
			name: "valid response for Kolkata timezone",
			input: `{
				"abbreviation": "IST",
				"client_ip": "203.0.113.1",
				"datetime": "2023-12-04T21:00:00.123456+05:30",
				"day_of_week": 1,
				"dst": false,
				"raw_offset": 19800,
				"timezone": "Asia/Kolkata",
				"unixtime": 1701694200,
				"utc_datetime": "2023-12-04T15:30:00.123456+00:00",
				"utc_offset": "+05:30",
				"week_number": 49
			}`,
			expectError: false,
			validate: func(t *testing.T, data WorldTimeData) {
				assert.Equal(t, "Asia/Kolkata", data.Timezone)
				assert.Equal(t, "+05:30", data.UTCOffset)
				assert.Equal(t, "IST", data.Abbreviation)
			},
		},
		{
			name:        "invalid json",
			input:       `{invalid json}`,
			expectError: true,
		},
		{
			name: "minimal valid response",
			input: `{
				"abbreviation": "UTC",
				"datetime": "2023-12-04T15:30:00.123456+00:00",
				"day_of_week": 1,
				"dst": false,
				"timezone": "UTC",
				"utc_datetime": "2023-12-04T15:30:00.123456+00:00",
				"utc_offset": "+00:00",
				"week_number": 49
			}`,
			expectError: false,
			validate: func(t *testing.T, data WorldTimeData) {
				assert.Equal(t, "UTC", data.Timezone)
				assert.Equal(t, "+00:00", data.UTCOffset)
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

			worldTimeData, ok := data.(WorldTimeData)
			require.True(t, ok, "expected WorldTimeData type")

			if tt.validate != nil {
				tt.validate(t, worldTimeData)
			}

			assert.WithinDuration(t, time.Now(), worldTimeData.FetchedAt, 5*time.Second)
		})
	}
}

func TestFetchData(t *testing.T) {
	tests := []struct {
		name     string
		timezone string
		wantURL  string
	}{
		{
			name:     "simple timezone",
			timezone: "America/New_York",
			wantURL:  "http://worldtimeapi.org/api/timezone/America/New_York",
		},
		{
			name:     "timezone with special chars",
			timezone: "America/Argentina/Buenos_Aires",
			wantURL:  "http://worldtimeapi.org/api/timezone/America/Argentina/Buenos_Aires",
		},
		{
			name:     "UTC timezone",
			timezone: "UTC",
			wantURL:  "http://worldtimeapi.org/api/timezone/UTC",
		},
	}

	cfg := &config.Config{
		DBWorldTime:         "test_db",
		WorldTimeAPIBaseURL: "http://worldtimeapi.org/api/timezone",
	}
	service := NewService(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := service.FetchData(ctx, tt.timezone)
			_ = err
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
		DBWorldTime:         "test_db",
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
			data: WorldTimeData{
				Timezone:     "America/New_York",
				UTCOffset:    "-05:00",
				CurrentTime:  time.Now(),
				DayOfWeek:    1,
				WeekNumber:   49,
				UTCDatetime:  time.Now().UTC(),
				IsDST:        false,
				Abbreviation: "EST",
				FetchedAt:    time.Now(),
			},
			validate: func(t *testing.T, client *mongo.Client, cfg *config.Config) {
				collection := client.Database(cfg.DBWorldTime).Collection(cfg.CollectionDailyData)
				var result WorldTimeData
				err := collection.FindOne(context.Background(), bson.M{"timezone": "America/New_York"}).Decode(&result)
				require.NoError(t, err)
				assert.Equal(t, "America/New_York", result.Timezone)
				assert.Equal(t, "EST", result.Abbreviation)
			},
		},
		{
			name:        "nil database client",
			db:          nil,
			data:        WorldTimeData{Timezone: "UTC"},
			expectError: true,
			errorMsg:    "mongo client is nil",
		},
		{
			name:        "invalid database client type",
			db:          "invalid",
			data:        WorldTimeData{Timezone: "UTC"},
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
		{
			name:        "invalid data type map",
			db:          client,
			data:        map[string]interface{}{"timezone": "UTC"},
			expectError: true,
			errorMsg:    "invalid data type",
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

func TestStoreData_MultipleTimezones(t *testing.T) {
	ctx := context.Background()

	mongoC, err := setupMongoContainer(ctx)
	require.NoError(t, err)
	defer mongoC.Terminate(ctx)

	client, err := setupTestDB(ctx, mongoC.URI, "test_db")
	require.NoError(t, err)
	defer client.Disconnect(ctx)

	cfg := &config.Config{
		DBWorldTime:         "test_db",
		CollectionDailyData: "daily_data",
	}
	service := NewService(cfg)

	tests := []struct {
		name      string
		timezones []WorldTimeData
		expected  int64
		verify    func(*testing.T, *mongo.Client, *config.Config)
	}{
		{
			name: "insert multiple timezones",
			timezones: []WorldTimeData{
				{
					Timezone:     "America/New_York",
					UTCOffset:    "-05:00",
					DayOfWeek:    1,
					WeekNumber:   49,
					IsDST:        false,
					Abbreviation: "EST",
					FetchedAt:    time.Now(),
				},
				{
					Timezone:     "Europe/London",
					UTCOffset:    "+00:00",
					DayOfWeek:    1,
					WeekNumber:   49,
					IsDST:        false,
					Abbreviation: "GMT",
					FetchedAt:    time.Now(),
				},
				{
					Timezone:     "Asia/Tokyo",
					UTCOffset:    "+09:00",
					DayOfWeek:    2,
					WeekNumber:   49,
					IsDST:        false,
					Abbreviation: "JST",
					FetchedAt:    time.Now(),
				},
				{
					Timezone:     "Australia/Sydney",
					UTCOffset:    "+11:00",
					DayOfWeek:    2,
					WeekNumber:   49,
					IsDST:        true,
					Abbreviation: "AEDT",
					FetchedAt:    time.Now(),
				},
				{
					Timezone:     "Asia/Kolkata",
					UTCOffset:    "+05:30",
					DayOfWeek:    1,
					WeekNumber:   49,
					IsDST:        false,
					Abbreviation: "IST",
					FetchedAt:    time.Now(),
				},
			},
			expected: 5,
			verify: func(t *testing.T, client *mongo.Client, cfg *config.Config) {
				collection := client.Database(cfg.DBWorldTime).Collection(cfg.CollectionDailyData)
				var result WorldTimeData
				err := collection.FindOne(context.Background(), bson.M{"timezone": "Asia/Tokyo"}).Decode(&result)
				require.NoError(t, err)
				assert.Equal(t, "Asia/Tokyo", result.Timezone)
				assert.Equal(t, "+09:00", result.UTCOffset)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, tz := range tt.timezones {
				err := service.StoreData(ctx, client, tz)
				require.NoError(t, err)
			}

			collection := client.Database(cfg.DBWorldTime).Collection(cfg.CollectionDailyData)
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
		DBWorldTime:         "test_db",
		CollectionDailyData: "daily_data",
	}
	service := NewService(cfg)

	tests := []struct {
		name          string
		numGoroutines int
		expected      int64
	}{
		{
			name:          "concurrent writes with 5 goroutines",
			numGoroutines: 5,
			expected:      5,
		},
		{
			name:          "concurrent writes with 10 goroutines",
			numGoroutines: 10,
			expected:      10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collection := client.Database(cfg.DBWorldTime).Collection(cfg.CollectionDailyData)
			collection.DeleteMany(ctx, bson.M{})

			timezones := []string{
				"America/New_York", "Europe/London", "Asia/Tokyo", "Australia/Sydney", "Asia/Kolkata",
				"Europe/Paris", "US/Pacific", "Asia/Shanghai", "America/Los_Angeles", "Europe/Berlin",
			}

			done := make(chan error, tt.numGoroutines)

			for i := 0; i < tt.numGoroutines; i++ {
				go func(id int) {
					data := WorldTimeData{
						Timezone:     timezones[id],
						UTCOffset:    fmt.Sprintf("%+d:00", -12+id%24),
						DayOfWeek:    (id % 7) + 1,
						WeekNumber:   49,
						IsDST:        id%2 == 0,
						Abbreviation: fmt.Sprintf("TZ%d", id),
						FetchedAt:    time.Now(),
					}
					done <- service.StoreData(ctx, client, data)
				}(i)
			}

			for i := 0; i < tt.numGoroutines; i++ {
				err := <-done
				assert.NoError(t, err)
			}

			collection = client.Database(cfg.DBWorldTime).Collection(cfg.CollectionDailyData)
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
		DBWorldTime:           "test_db",
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
			name: "with seeded timezone params",
			setup: func() {
				err := seedTimezoneFetchParams(ctx, client, cfg.DBWorldTime, cfg.CollectionFetchParams)
				require.NoError(t, err)
			},
			clientInput: client,
			expectError: false,
		},
		{
			name: "invalid client type",
			setup: func() {
			},
			clientInput: "invalid",
			expectError: true,
			errorMsg:    "expected *mongo.Client",
		},
		{
			name: "verify fetch params loaded",
			setup: func() {
			},
			clientInput: client,
			expectError: false,
			verify: func(t *testing.T, client *mongo.Client, cfg *config.Config) {
				collection := client.Database(cfg.DBWorldTime).Collection(cfg.CollectionFetchParams)
				count, err := collection.CountDocuments(context.Background(), bson.M{})
				require.NoError(t, err)
				assert.Equal(t, int64(5), count)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			service := NewService(cfg)
			ch := &channels.Channels{
				DataRequest: make(chan models.DataRequest, 10),
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

			require.NoError(t, err)
			close(ch.DataRequest)

			if tt.verify != nil {
				tt.verify(t, client, cfg)
			}
		})
	}
}

func TestIntegration_ParseAndStore(t *testing.T) {
	ctx := context.Background()

	mongoC, err := setupMongoContainer(ctx)
	require.NoError(t, err)
	defer mongoC.Terminate(ctx)

	client, err := setupTestDB(ctx, mongoC.URI, "test_db")
	require.NoError(t, err)
	defer client.Disconnect(ctx)

	cfg := &config.Config{
		DBWorldTime:         "test_db",
		CollectionDailyData: "daily_data",
	}
	service := NewService(cfg)

	jsonInput := `{
		"abbreviation": "JST",
		"client_ip": "203.0.113.1",
		"datetime": "2023-12-05T00:30:00.123456+09:00",
		"day_of_week": 2,
		"dst": false,
		"raw_offset": 32400,
		"timezone": "Asia/Tokyo",
		"unixtime": 1701694200,
		"utc_datetime": "2023-12-04T15:30:00.123456+00:00",
		"utc_offset": "+09:00",
		"week_number": 49
	}`

	parsedData, err := service.ParseData([]byte(jsonInput))
	require.NoError(t, err)

	err = service.StoreData(ctx, client, parsedData)
	require.NoError(t, err)

	collection := client.Database(cfg.DBWorldTime).Collection(cfg.CollectionDailyData)
	var result WorldTimeData
	err = collection.FindOne(ctx, bson.M{"timezone": "Asia/Tokyo"}).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "Asia/Tokyo", result.Timezone)
	assert.Equal(t, "+09:00", result.UTCOffset)
	assert.Equal(t, "JST", result.Abbreviation)
	assert.Equal(t, 2, result.DayOfWeek)
	assert.Equal(t, 49, result.WeekNumber)
	assert.False(t, result.IsDST)
}
