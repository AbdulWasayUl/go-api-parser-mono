package db

import (
	"context"
	"time"

	"github.com/AbdulWasayUl/go-api-parser-mono/internal/config"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/db/migrations"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/logger"
	"github.com/AbdulWasayUl/go-api-parser-mono/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const migrationCollectionName = "migrations_history"

func ConnectMongoDB(ctx context.Context, cfg *config.Config) (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(cfg.MongoURI).SetAuth(options.Credential{
		Username:   "admin",
		Password:   "password",
		AuthSource: cfg.MongoAuthDB,
	})

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err = client.Ping(ctxTimeout, nil)
	if err != nil {
		return nil, err
	}

	logger.Info("Successfully connected to MongoDB!")
	return client, nil
}

func DisconnectMongoDB(ctx context.Context, client *mongo.Client) error {
	if err := client.Disconnect(ctx); err != nil {
		return err
	}
	logger.Info("Disconnected from MongoDB.")
	return nil
}

func RunMigrations(ctx context.Context, client *mongo.Client, cfg *config.Config) error {
	migrations := []models.Migration{
		{Name: "initial_data_weather", Func: migrations.MigrateWeatherData(cfg)},
		{Name: "initial_data_openaq", Func: migrations.MigrateOpenAQData(cfg)},
		{Name: "initial_data_worldtime", Func: migrations.MigrateWorldTimeData(cfg)},
		{Name: "initial_data_restcountries", Func: migrations.MigrateRestCountriesData(cfg)},
	}

	db := client.Database(cfg.DBWeather)

	coll := db.Collection(migrationCollectionName)

	for _, m := range migrations {
		var result struct{ Name string }
		err := coll.FindOne(ctx, bson.M{"name": m.Name}).Decode(&result)
		if err == mongo.ErrNoDocuments {
			logger.Info("Running migration: %s", m.Name)
			if err := m.Func(ctx, client); err != nil {
				logger.Error("Error applying migration %s: %v", m.Name, err)
				return err
			}
			_, err = coll.InsertOne(ctx, bson.M{"name": m.Name, "applied_at": time.Now()})
			if err != nil {
				return err
			}
			logger.Info("Migration %s applied successfully.", m.Name)
		} else if err != nil {
			return err
		} else {
			logger.Info("Migration %s already applied, skipping.", m.Name)
		}
	}

	return nil
}

func GetFetchParams(ctx context.Context, client interface{}, dbName, collectionName string) ([]map[string]interface{}, error) {
	mongoClient, _ := client.(*mongo.Client)

	db := mongoClient.Database(dbName)
	coll := db.Collection(collectionName)

	cursor, err := coll.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}
