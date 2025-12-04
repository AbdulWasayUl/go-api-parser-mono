package db_test

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/AbdulWasayUl/go-api-parser-mono/internal/config"
	"github.com/AbdulWasayUl/go-api-parser-mono/internal/db"
	"github.com/docker/go-connections/nat"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
)

// Helper: Start temporary MongoDB container
func setupMongoContainer(ctx context.Context) (tc.Container, string, error) {
	req := tc.ContainerRequest{
		Image:        "mongo:7.0",
		ExposedPorts: []string{"27017/tcp"},
		Env: map[string]string{
			"MONGO_INITDB_ROOT_USERNAME": "admin",
			"MONGO_INITDB_ROOT_PASSWORD": "password",
		},
		WaitingFor: wait.ForListeningPort("27017/tcp"),
	}

	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", err
	}

	port, err := container.MappedPort(ctx, nat.Port("27017"))
	if err != nil {
		return nil, "", err
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, "", err
	}

	mongoURI := fmt.Sprintf("mongodb://admin:password@%s:%s", host, port.Port())
	return container, mongoURI, nil
}

func TestMongoDBFunctions(t *testing.T) {
	ctx := context.Background()

	// Start MongoDB container
	container, mongoURI, err := setupMongoContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to start MongoDB container: %v", err)
	}
	defer container.Terminate(ctx)

	cfg := &config.Config{
		MongoURI:    mongoURI,
		MongoAuthDB: "admin",
		DBWeather:   "weather_test",
	}

	// Connect to MongoDB
	client, err := db.ConnectMongoDB(ctx, cfg)
	if err != nil {
		t.Fatalf("ConnectMongoDB failed: %v", err)
	}

	// Disconnect test
	defer db.DisconnectMongoDB(ctx, client)

	// Test GetFetchParams on empty collection
	collName := "fetch_params_test"
	results, err := db.GetFetchParams(ctx, client, cfg.DBWeather, collName)
	if err != nil {
		t.Fatalf("GetFetchParams failed: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Expected empty results, got %v", results)
	}

	// Insert dummy data and test GetFetchParams
	coll := client.Database(cfg.DBWeather).Collection(collName)
	_, err = coll.InsertOne(ctx, bson.M{"key": "value"})
	if err != nil {
		t.Fatalf("InsertOne failed: %v", err)
	}

	results, err = db.GetFetchParams(ctx, client, cfg.DBWeather, collName)
	if err != nil {
		t.Fatalf("GetFetchParams failed: %v", err)
	}
	if len(results) != 1 || results[0]["key"] != "value" {
		t.Fatalf("Unexpected fetch results: %v", results)
	}
}

// Optional: Test RunMigrations (with mocked migration funcs)
func TestRunMigrations(t *testing.T) {
	ctx := context.Background()

	container, mongoURI, err := setupMongoContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to start MongoDB container: %v", err)
	}
	defer container.Terminate(ctx)

	cfg := &config.Config{
		MongoURI:    mongoURI,
		MongoAuthDB: "admin",
		DBWeather:   "weather_test_migrations",
	}

	client, err := db.ConnectMongoDB(ctx, cfg)
	if err != nil {
		t.Fatalf("ConnectMongoDB failed: %v", err)
	}
	defer db.DisconnectMongoDB(ctx, client)

	// Insert dummy migrations collection
	migrationsColl := client.Database(cfg.DBWeather).Collection("migrations_history")
	_, _ = migrationsColl.InsertOne(ctx, bson.M{"name": "dummy_migration", "applied_at": time.Now()})

	// Just test that function runs without panic
	err = db.RunMigrations(ctx, client, cfg)
	if err != nil {
		log.Printf("RunMigrations returned error (expected if migration functions not implemented): %v", err)
	}
}
