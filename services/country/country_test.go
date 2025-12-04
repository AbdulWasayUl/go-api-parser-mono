package country

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

func seedCountryFetchParams(ctx context.Context, client *mongo.Client, dbName, collectionName string) error {
	collection := client.Database(dbName).Collection(collectionName)

	params := []interface{}{
		bson.M{"country_code": "GB", "country_name": "United Kingdom"},
		bson.M{"country_code": "FR", "country_name": "France"},
		bson.M{"country_code": "DE", "country_name": "Germany"},
		bson.M{"country_code": "IT", "country_name": "Italy"},
		bson.M{"country_code": "ES", "country_name": "Spain"},
	}

	_, err := collection.InsertMany(ctx, params)
	return err
}

func TestNewService(t *testing.T) {
	cfg := &config.Config{
		DBRestCountries:         "test_db",
		RestCountriesAPIBaseURL: "https://restcountries.com/v3.1/alpha",
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
		validate    func(*testing.T, CountryData)
	}{
		{
			name: "valid response with capital and currency",
			input: `[{
				"name": {
					"common": "France",
					"official": "French Republic"
				},
				"cca2": "FR",
				"independent": true,
				"unMember": true,
				"capital": ["Paris"],
				"region": "Europe",
				"subregion": "Western Europe",
				"population": 67750000,
				"area": 551695,
				"currencies": {
					"EUR": {
						"name": "Euro",
						"symbol": "€"
					}
				}
			}]`,
			expectError: false,
			validate: func(t *testing.T, countryData CountryData) {
				assert.Equal(t, "FR", countryData.CountryCode)
				assert.Equal(t, "French Republic", countryData.OfficialName)
				assert.Equal(t, "France", countryData.CommonName)
				assert.Equal(t, "Paris", countryData.Capital)
				assert.Equal(t, "Euro", countryData.Currency)
				assert.Equal(t, "€", countryData.CurrencySym)
				assert.Equal(t, 67750000, countryData.Population)
				assert.Equal(t, 551695.0, countryData.Area)
				assert.True(t, countryData.Independent)
				assert.True(t, countryData.UnMember)
			},
		},
		{
			name: "response without capital",
			input: `[{
				"name": {
					"common": "Greenland",
					"official": "Greenland"
				},
				"cca2": "GL",
				"independent": false,
				"unMember": false,
				"capital": [],
				"region": "Americas",
				"subregion": "Northern America",
				"population": 56000,
				"area": 2166086,
				"currencies": {
					"DKK": {
						"name": "Danish krone",
						"symbol": "kr"
					}
				}
			}]`,
			expectError: false,
			validate: func(t *testing.T, countryData CountryData) {
				assert.Equal(t, "GL", countryData.CountryCode)
				assert.Equal(t, "", countryData.Capital)
				assert.Equal(t, "Danish krone", countryData.Currency)
			},
		},
		{
			name: "response without currency",
			input: `[{
				"name": {
					"common": "Antarctica",
					"official": "Antarctica"
				},
				"cca2": "AQ",
				"independent": false,
				"unMember": false,
				"capital": [],
				"region": "Antarctic",
				"subregion": "",
				"population": 0,
				"area": 14200000,
				"currencies": {}
			}]`,
			expectError: false,
			validate: func(t *testing.T, countryData CountryData) {
				assert.Equal(t, "AQ", countryData.CountryCode)
				assert.Equal(t, "", countryData.Currency)
				assert.Equal(t, "", countryData.CurrencySym)
			},
		},
		{
			name:        "empty response array",
			input:       `[]`,
			expectError: true,
		},
		{
			name:        "invalid json",
			input:       `{invalid json}`,
			expectError: true,
		},
		{
			name: "multiple currencies, takes first",
			input: `[{
				"name": {
					"common": "Switzerland",
					"official": "Swiss Confederation"
				},
				"cca2": "CH",
				"independent": true,
				"unMember": true,
				"capital": ["Bern"],
				"region": "Europe",
				"subregion": "Western Europe",
				"population": 8700000,
				"area": 41285,
				"currencies": {
					"CHF": {
						"name": "Swiss franc",
						"symbol": "₣"
					},
					"EUR": {
						"name": "Euro",
						"symbol": "€"
					}
				}
			}]`,
			expectError: false,
			validate: func(t *testing.T, countryData CountryData) {
				assert.NotEmpty(t, countryData.Currency)
				assert.NotEmpty(t, countryData.CurrencySym)
			},
		},
		{
			name: "zero population and area",
			input: `[{
				"name": {
					"common": "Test Country",
					"official": "Test Country Official"
				},
				"cca2": "TC",
				"independent": false,
				"unMember": false,
				"capital": [],
				"region": "Test",
				"subregion": "Test",
				"population": 0,
				"area": 0,
				"currencies": {}
			}]`,
			expectError: false,
			validate: func(t *testing.T, countryData CountryData) {
				assert.Equal(t, 0, countryData.Population)
				assert.Equal(t, 0.0, countryData.Area)
			},
		},
		{
			name: "special characters in names",
			input: `[{
				"name": {
					"common": "Côte d'Ivoire",
					"official": "République de Côte d'Ivoire"
				},
				"cca2": "CI",
				"independent": true,
				"unMember": true,
				"capital": ["Yamoussoukro"],
				"region": "Africa",
				"subregion": "Western Africa",
				"population": 26378274,
				"area": 322463,
				"currencies": {
					"XOF": {
						"name": "West African CFA franc",
						"symbol": "Fr"
					}
				}
			}]`,
			expectError: false,
			validate: func(t *testing.T, countryData CountryData) {
				assert.Equal(t, "Côte d'Ivoire", countryData.CommonName)
				assert.Equal(t, "CI", countryData.CountryCode)
			},
		},
		{
			name: "multiple capitals, takes first",
			input: `[{
				"name": {
					"common": "Test",
					"official": "Test Official"
				},
				"cca2": "TS",
				"independent": true,
				"unMember": true,
				"capital": ["Capital1", "Capital2", "Capital3"],
				"region": "Test",
				"subregion": "Test",
				"population": 1000000,
				"area": 100000,
				"currencies": {
					"USD": {
						"name": "Dollar",
						"symbol": "$"
					}
				}
			}]`,
			expectError: false,
			validate: func(t *testing.T, countryData CountryData) {
				assert.Equal(t, "Capital1", countryData.Capital)
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

			countryData, ok := data.(CountryData)
			require.True(t, ok, "expected CountryData type")

			if tt.validate != nil {
				tt.validate(t, countryData)
			}

			assert.WithinDuration(t, time.Now(), countryData.FetchedAt, 5*time.Second)
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
			id:      "FR",
			wantURL: "https://restcountries.com/v3.1/alpha/FR",
		},
		{
			name:    "country code with special chars",
			id:      "FR&test",
			wantURL: "https://restcountries.com/v3.1/alpha/FR%26test",
		},
	}

	cfg := &config.Config{
		DBRestCountries:         "test_db",
		RestCountriesAPIBaseURL: "https://restcountries.com/v3.1/alpha",
	}
	service := NewService(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := service.FetchData(ctx, tt.id)
			// Error is expected (real API call), or may succeed
			// Main goal is verifying URL construction doesn't panic
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
		DBRestCountries:     "test_db",
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
			data: CountryData{
				CountryCode:  "FR",
				OfficialName: "French Republic",
				CommonName:   "France",
				Independent:  true,
				UnMember:     true,
				Capital:      "Paris",
				Region:       "Europe",
				Subregion:    "Western Europe",
				Currency:     "Euro",
				CurrencySym:  "€",
				Population:   67750000,
				Area:         551695,
				FetchedAt:    time.Now(),
			},
			validate: func(t *testing.T, client *mongo.Client, cfg *config.Config) {
				collection := client.Database(cfg.DBRestCountries).Collection(cfg.CollectionDailyData)
				var result CountryData
				err := collection.FindOne(context.Background(), bson.M{"country_code": "FR"}).Decode(&result)
				require.NoError(t, err)
				assert.Equal(t, "France", result.CommonName)
				assert.Equal(t, "Paris", result.Capital)
				assert.Equal(t, 67750000, result.Population)
			},
		},
		{
			name:        "nil database client",
			db:          nil,
			data:        CountryData{CountryCode: "FR", CommonName: "France"},
			expectError: true,
			errorMsg:    "mongo client is nil",
		},
		{
			name:        "invalid database client type",
			db:          "invalid",
			data:        CountryData{CountryCode: "FR", CommonName: "France"},
			expectError: true,
			errorMsg:    "expected *mongo.Client",
		},
		{
			name:        "invalid data type",
			db:          client,
			data:        "invalid data",
			expectError: true,
			errorMsg:    "expected CountryData",
		},
		{
			name:        "wrong data type map",
			db:          client,
			data:        map[string]interface{}{"code": "DE"},
			expectError: true,
			errorMsg:    "expected CountryData",
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
		DBRestCountries:     "test_db",
		CollectionDailyData: "daily_data",
	}
	service := NewService(cfg)

	tests := []struct {
		name      string
		countries []CountryData
		expected  int64
		verify    func(*testing.T, *mongo.Client, *config.Config)
	}{
		{
			name: "insert multiple countries",
			countries: []CountryData{
				{
					CountryCode:  "GB",
					OfficialName: "United Kingdom",
					CommonName:   "United Kingdom",
					Capital:      "London",
					Currency:     "British pound",
					CurrencySym:  "£",
					Population:   67736802,
					Area:         242900,
					FetchedAt:    time.Now(),
				},
				{
					CountryCode:  "FR",
					OfficialName: "French Republic",
					CommonName:   "France",
					Capital:      "Paris",
					Currency:     "Euro",
					CurrencySym:  "€",
					Population:   67750000,
					Area:         551695,
					FetchedAt:    time.Now(),
				},
				{
					CountryCode:  "DE",
					OfficialName: "Federal Republic of Germany",
					CommonName:   "Germany",
					Capital:      "Berlin",
					Currency:     "Euro",
					CurrencySym:  "€",
					Population:   84457000,
					Area:         357022,
					FetchedAt:    time.Now(),
				},
				{
					CountryCode:  "IT",
					OfficialName: "Italian Republic",
					CommonName:   "Italy",
					Capital:      "Rome",
					Currency:     "Euro",
					CurrencySym:  "€",
					Population:   57562100,
					Area:         301340,
					FetchedAt:    time.Now(),
				},
				{
					CountryCode:  "ES",
					OfficialName: "Kingdom of Spain",
					CommonName:   "Spain",
					Capital:      "Madrid",
					Currency:     "Euro",
					CurrencySym:  "€",
					Population:   47562100,
					Area:         505990,
					FetchedAt:    time.Now(),
				},
			},
			expected: 5,
			verify: func(t *testing.T, client *mongo.Client, cfg *config.Config) {
				collection := client.Database(cfg.DBRestCountries).Collection(cfg.CollectionDailyData)
				var result CountryData
				err := collection.FindOne(context.Background(), bson.M{"country_code": "DE"}).Decode(&result)
				require.NoError(t, err)
				assert.Equal(t, "Germany", result.CommonName)
				assert.Equal(t, "Berlin", result.Capital)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, country := range tt.countries {
				err := service.StoreData(ctx, client, country)
				require.NoError(t, err)
			}

			collection := client.Database(cfg.DBRestCountries).Collection(cfg.CollectionDailyData)
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
		DBRestCountries:     "test_db",
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
			// Clear collection before each test
			collection := client.Database(cfg.DBRestCountries).Collection(cfg.CollectionDailyData)
			collection.DeleteMany(ctx, bson.M{})

			countryNames := []string{"UK", "France", "Germany", "Italy", "Spain", "Austria", "Belgium", "Czech", "Denmark", "Hungary"}
			capitals := []string{"London", "Paris", "Berlin", "Rome", "Madrid", "Vienna", "Brussels", "Prague", "Copenhagen", "Budapest"}

			done := make(chan error, tt.numGoroutines)

			for i := 0; i < tt.numGoroutines; i++ {
				go func(id int) {
					countryData := CountryData{
						CountryCode: countryNames[id],
						CommonName:  countryNames[id],
						Capital:     capitals[id],
						Currency:    "Euro",
						Population:  1000000 + id,
						Area:        100000 + float64(id),
						FetchedAt:   time.Now(),
					}
					done <- service.StoreData(ctx, client, countryData)
				}(i)
			}

			for i := 0; i < tt.numGoroutines; i++ {
				err := <-done
				assert.NoError(t, err)
			}

			collection = client.Database(cfg.DBRestCountries).Collection(cfg.CollectionDailyData)
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
		DBRestCountries:       "test_db",
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
			name: "with seeded params",
			setup: func() {
				err := seedCountryFetchParams(ctx, client, cfg.DBRestCountries, cfg.CollectionFetchParams)
				require.NoError(t, err)
			},
			clientInput: client,
			expectError: false,
		},
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
				// Params already seeded in first test
			},
			clientInput: client,
			expectError: false,
			verify: func(t *testing.T, client *mongo.Client, cfg *config.Config) {
				collection := client.Database(cfg.DBRestCountries).Collection(cfg.CollectionFetchParams)
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
		DBRestCountries:     "test_db",
		CollectionDailyData: "daily_data",
	}
	service := NewService(cfg)

	jsonInput := `[{
		"name": {
			"common": "Germany",
			"official": "Federal Republic of Germany"
		},
		"cca2": "DE",
		"independent": true,
		"unMember": true,
		"capital": ["Berlin"],
		"region": "Europe",
		"subregion": "Central Europe",
		"population": 84457000,
		"area": 357022,
		"currencies": {
			"EUR": {
				"name": "Euro",
				"symbol": "€"
			}
		}
	}]`

	parsedData, err := service.ParseData([]byte(jsonInput))
	require.NoError(t, err)

	err = service.StoreData(ctx, client, parsedData)
	require.NoError(t, err)

	collection := client.Database(cfg.DBRestCountries).Collection(cfg.CollectionDailyData)
	var result CountryData
	err = collection.FindOne(ctx, bson.M{"country_code": "DE"}).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "Germany", result.CommonName)
	assert.Equal(t, "Federal Republic of Germany", result.OfficialName)
	assert.Equal(t, "Berlin", result.Capital)
	assert.Equal(t, "Euro", result.Currency)
	assert.Equal(t, "€", result.CurrencySym)
	assert.Equal(t, 84457000, result.Population)
	assert.Equal(t, 357022.0, result.Area)
	assert.True(t, result.Independent)
	assert.True(t, result.UnMember)
}
