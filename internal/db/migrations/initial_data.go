package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AbdulWasayUl/go-api-parser-mono/internal/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	dataDirPath          = "internal/db/migrations/data"
	fetchParamCollection = "fetch_params"
	dailyDataCollection  = "daily_data"
)

func loadJSONData(fileName string) ([]interface{}, error) {

	filePath := filepath.Join(dataDirPath, fileName)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	var rawData []map[string]interface{}
	if err := json.Unmarshal(data, &rawData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON data from %s: %v", filePath, err)
	}

	var documents []interface{}
	for _, item := range rawData {
		documents = append(documents, bson.M(item))
	}

	return documents, nil
}

func runMigrationHelper(ctx context.Context, client *mongo.Client, dbName, filename string) error {
	db := client.Database(dbName)
	coll := db.Collection(fetchParamCollection)

	// Create collection, ignore if it already exists
	if err := db.CreateCollection(ctx, dailyDataCollection); err != nil {
		if !mongo.IsDuplicateKeyError(err) && err.Error() != "namespace already exists" {
			return fmt.Errorf("failed to create daily data collection: %w", err)
		}
	}

	// Load data from JSON file
	data, err := loadJSONData(filename)
	if err != nil {
		return fmt.Errorf("failed to load JSON data from %s: %w", filename, err)
	}

	// Insert data into collection
	if _, err := coll.InsertMany(ctx, data); err != nil {
		return fmt.Errorf("failed to insert data for %s: %w", dbName, err)
	}

	return nil
}

func MigrateWeatherData(cfg *config.Config) func(ctx context.Context, client *mongo.Client) error {
	return func(ctx context.Context, client *mongo.Client) error {
		return runMigrationHelper(ctx, client, cfg.DBWeather, "weather_params.json")
	}
}

func MigrateOpenAQData(cfg *config.Config) func(ctx context.Context, client *mongo.Client) error {
	return func(ctx context.Context, client *mongo.Client) error {
		return runMigrationHelper(ctx, client, cfg.DBOpenAQ, "openaq_params.json")
	}
}

func MigrateWorldTimeData(cfg *config.Config) func(ctx context.Context, client *mongo.Client) error {
	return func(ctx context.Context, client *mongo.Client) error {
		return runMigrationHelper(ctx, client, cfg.DBWorldTime, "worldtime_params.json")
	}
}

func MigrateRestCountriesData(cfg *config.Config) func(ctx context.Context, client *mongo.Client) error {
	return func(ctx context.Context, client *mongo.Client) error {
		return runMigrationHelper(ctx, client, cfg.DBRestCountries, "restcountries_params.json")
	}
}
