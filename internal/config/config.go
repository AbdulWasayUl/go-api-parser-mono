package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds the application configuration
type Config struct {
	WeatherAPIKey               string
	OpenAQAPIKey                string
	MongoURI                    string
	MongoAuthDB                 string
	DBWeather                   string
	DBOpenAQ                    string
	DBWorldTime                 string
	DBRestCountries             string
	CollectionFetchParams       string
	CollectionDailyData         string
	CollectionMigrationsHistory string
	WeatherAPIBaseURL           string
	OpenAQAPIBaseURL            string
	WorldTimeAPIBaseURL         string
	RestCountriesAPIBaseURL     string
}

// Load reads the .env file and loads the configuration
func Load() *Config {
	err := godotenv.Load()
	// Ignore err if .env file is not found in deployment
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	return &Config{
		WeatherAPIKey:               os.Getenv("WEATHER_API_KEY"),
		OpenAQAPIKey:                os.Getenv("OPENAQ_API_KEY"),
		MongoURI:                    getMongoURI(),
		MongoAuthDB:                 os.Getenv("MONGO_AUTH_DB"),
		DBWeather:                   os.Getenv("DB_WEATHER_NAME"),
		DBOpenAQ:                    os.Getenv("DB_OPENAQ_NAME"),
		DBWorldTime:                 os.Getenv("DB_WORLDTIME_NAME"),
		DBRestCountries:             os.Getenv("DB_RESTCOUNTRIES_NAME"),
		CollectionFetchParams:       os.Getenv("COLLECTION_FETCH_PARAMS"),
		CollectionDailyData:         os.Getenv("COLLECTION_DAILY_DATA"),
		CollectionMigrationsHistory: os.Getenv("COLLECTION_MIGRATIONS_HISTORY"),
		WeatherAPIBaseURL:           os.Getenv("WEATHER_API_BASE_URL"),
		OpenAQAPIBaseURL:            os.Getenv("OPENAQ_API_BASE_URL"),
		WorldTimeAPIBaseURL:         os.Getenv("WORLDTIME_API_BASE_URL"),
		RestCountriesAPIBaseURL:     os.Getenv("RESTCOUNTRIES_API_BASE_URL"),
	}
}

// getMongoURI constructs the MongoDB URI from environment variables
func getMongoURI() string {
	host := os.Getenv("MONGO_HOST")
	port := os.Getenv("MONGO_PORT")
	user := os.Getenv("MONGO_USER")
	pass := os.Getenv("MONGO_PASS")

	return "mongodb://" + user + ":" + pass + "@" + host + ":" + port
}
