# Go API Parser Mono

A concurrent, production-ready Go application that fetches, parses, and stores data from multiple external APIs using MongoDB as the data store. The application demonstrates advanced Go patterns including worker pools, goroutine scheduling, channel-based pipelines, and comprehensive testing.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Running the Application](#running-the-application)
- [Testing](#testing)
- [Project Structure](#project-structure)
- [Services](#services)
- [API Keys](#api-keys)

---

## Overview

This project aggregates real-time data from four external APIs:

1. **Weather API** — Current weather data by city name
2. **Open AQ API** — Air quality data by country
3. **World Time API** — Time zone and UTC offset information
4. **REST Countries API** — Country metadata

All data is fetched concurrently using a worker pool pattern, parsed into structured Go types, and stored in MongoDB. A cron scheduler runs batch jobs at configurable intervals.

---

## Features

✅ **Concurrent Data Fetching** — Worker pool with goroutines for high-throughput API calls  
✅ **Channel-based Pipelines** — Structured data flow between fetch, parse, and store stages  
✅ **Scheduled Jobs** — Cron-based batch execution using gocron  
✅ **MongoDB Integration** — BSON serialization and efficient document storage  
✅ **Comprehensive Testing** — Table-driven unit and integration tests with 85%+ coverage  
✅ **Testcontainers** — Containerized integration testing with ephemeral MongoDB instances  
✅ **Structured Logging** — Timestamped, leveled logging (INFO, ERROR)  
✅ **Docker Support** — Multi-stage Dockerfile and docker-compose for easy deployment  

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Entry Point                  │
│                      (cmd/app/main.go)                      │
└──────────────────────────┬──────────────────────────────────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
   ┌────▼────┐        ┌───▼────┐        ┌───▼──────┐
   │ Scheduler│        │Workpool│        │ Channels │
   │ (Cron)   │        │(Workers)│       │(Pipeline)│
   └────┬────┘        └───┬────┘        └────┬─────┘
        │                  │                  │
        └──────────────────┼──────────────────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
   ┌────▼────────┐   ┌────▼─────┐   ┌───────▼──┐
   │   Services  │   │   Fetch   │   │  Parse   │
   │  (4 APIs)   │   │   Data    │   │   Data   │
   └────┬────────┘   └────┬─────┘   └───────┬──┘
        │                  │                  │
        └──────────────────┼──────────────────┘
                           │
                      ┌────▼──────┐
                      │   Store    │
                      │ (MongoDB)  │
                      └────────────┘
```

**Key Components:**

- **Scheduler** — Manages cron jobs for periodic batch execution
- **Workpool** — Distributes API requests across worker goroutines
- **Channels** — Decouples fetch, parse, and storage operations
- **Services** — Encapsulate API-specific fetch, parse, and store logic
- **Database** — MongoDB for persistence with migrations

---

## Prerequisites

- **Go** 1.24.0 or later
- **Docker** & **Docker Compose** (for containerized MongoDB)
- **MongoDB** 6.0+ (running locally or via Docker)
- **Environment Variables** — API keys (see [Configuration](#configuration))

### Optional

- **Make** — For running common tasks (build, test, run)

---

## Installation

### 1. Clone the Repository

```bash
git clone https://github.com/AbdulWasayUl/go-api-parser-mono.git
cd go-api-parser-mono
```

### 2. Install Dependencies

```bash
go mod download
go mod tidy
```

### 3. Start MongoDB

**Option A: Docker Compose (Recommended)**

```bash
docker-compose up -d
```

This starts MongoDB on `localhost:27017` with credentials:
- **Username:** `admin`
- **Password:** `password`

**Option B: Local MongoDB Installation**

Ensure MongoDB is running on port `27017`.

---

## Configuration

Create a `.env` file in the project root with the following variables:

```env
# MongoDB
MONGODB_URI=mongodb://admin:password@localhost:27017
DB_NAME=api_data

# API Keys (obtain from respective services)
WEATHER_API_KEY=your_weatherapi_key_here
WEATHER_API_BASE_URL=https://api.weatherapi.com/v1/current.json

OPENAQ_API_BASE_URL=https://api.openaq.org/v3/countries

WORLDTIME_API_BASE_URL=http://worldtimeapi.org/api/timezone

RESTCOUNTRIES_API_BASE_URL=https://restcountries.com/v3.1/all

# Database Collections
DB_WEATHER=weather_data
DB_AQI=aqi_data
DB_WORLD_TIME=time_data
DB_COUNTRY=country_data

COLLECTION_DAILY_DATA=daily_data
COLLECTION_FETCH_PARAMS=fetch_params
```

### Getting API Keys

- **Weather API:** [weatherapi.com](https://www.weatherapi.com/) (free tier available)
- **Open AQ:** [openaq.org](https://openaq.org/) (free, no key required)
- **World Time API:** [worldtimeapi.org](http://worldtimeapi.org/) (free, no key required)
- **REST Countries:** [restcountries.com](https://restcountries.com/) (free, no key required)

---

## Running the Application

### Local Development

```bash
# Build the application
go build -o app cmd/app/main.go

# Run the application
./app
```

The application will:
1. Initialize the logger
2. Load environment configuration
3. Connect to MongoDB
4. Initialize the scheduler with cron jobs
5. Start listening for API requests and scheduled batch jobs

Press `Ctrl+C` to gracefully shut down.

### Using Docker Compose

```bash
docker-compose up
```

This builds the Go application image and starts both the application and MongoDB containers.

### With Make (if available)

```bash
make run          # Build and run locally
make docker-up    # Start with Docker Compose
make docker-down  # Stop Docker Compose
```

---

## Testing

### Run All Tests

```bash
go test ./...
```

### Run Tests with Coverage Report

```bash
go test -cover ./...
```

**Current Coverage:**

- `internal/scheduler` — 92%
- `internal/channels` — 100%
- `internal/workpool` — 100%
- `services/weather` — 79%
- `services/aqi` — 86%
- `services/country` — 89%
- `services/time` — 88%
- `internal/api` — 87%
- `internal/db` — 71%
- `internal/logger` — 55%

### Run Specific Package Tests

```bash
# Test weather service
go test -v ./services/weather

# Test scheduler
go test -v ./internal/scheduler

# Test with verbose output
go test -v ./...
```

### Run Tests with Verbose Output and Custom Timeout

```bash
go test -v -timeout 30s ./...
```

### Integration Tests

Integration tests use **Testcontainers** to spin up ephemeral MongoDB containers. Ensure Docker is running:

```bash
# Run integration tests (requires Docker)
docker ps  # Verify Docker is running
go test -v ./services/weather ./services/aqi ./services/country ./services/time
```

### Unit Tests Only (No Docker Required)

Some tests are table-driven and mock-based:

```bash
go test -v ./internal/scheduler
go test -v ./internal/workpool
```

---

## Project Structure

```
go-api-parser-mono/
├── cmd/
│   └── app/
│       └── main.go              # Application entry point
├── internal/
│   ├── api/
│   │   ├── client.go            # HTTP client for API requests
│   │   └── client_test.go
│   ├── channels/
│   │   ├── channels.go          # Pipeline channel definitions
│   │   └── channels_test.go
│   ├── config/
│   │   └── config.go            # Environment configuration loader
│   ├── db/
│   │   ├── db.go                # MongoDB connection and operations
│   │   ├── db_test.go
│   │   └── migrations/
│   │       ├── initial_data.go
│   │       └── data/             # JSON parameter files
│   ├── logger/
│   │   ├── logger.go            # Structured logging
│   │   └── logger_test.go
│   ├── scheduler/
│   │   ├── scheduler.go         # Cron job scheduler
│   │   └── scheduler_test.go
│   └── workpool/
│       ├── workpool.go          # Worker pool implementation
│       └── workpool_test.go
├── services/
│   ├── weather/
│   │   ├── weather.go
│   │   ├── weather_test.go
│   │   └── models.go
│   ├── aqi/
│   │   ├── aqi.go
│   │   ├── aqi_test.go
│   │   └── models.go
│   ├── country/
│   │   ├── country.go
│   │   ├── country_test.go
│   │   └── models.go
│   └── time/
│       ├── worldtime.go
│       ├── worldtime_test.go
│       └── models.go
├── models/
│   └── common.go                # Shared data structures
├── .env                          # Environment variables (create this)
├── docker-compose.yml           # Docker Compose configuration
├── Dockerfile                   # Multi-stage build
├── go.mod                       # Go module definition
├── go.sum                       # Dependency checksums
├── architecture.txt             # Architecture notes
└── README.md                    # This file
```

---

## Services

### 1. Weather Service

**Source:** [WeatherAPI.com](https://www.weatherapi.com/)

**Functionality:**
- Fetches current weather data by city name
- Parses JSON response into `WeatherData` struct
- Stores temperature, conditions, wind, pressure, and precipitation

**Key Files:**
- `services/weather/weather.go`
- `services/weather/models.go`
- `services/weather/weather_test.go`

### 2. Air Quality (AQI) Service

**Source:** [OpenAQ API](https://api.openaq.org)

**Functionality:**
- Fetches air quality measurements by country
- Parses PM2.5, PM10, and other pollutant levels
- Stores city-level air quality data

**Key Files:**
- `services/aqi/aqi.go`
- `services/aqi/models.go`
- `services/aqi/aqi_test.go`

### 3. World Time Service

**Source:** [World Time API](http://worldtimeapi.org/)

**Functionality:**
- Fetches current time and timezone information
- Parses UTC offset and daylight saving time details
- Stores timezone metadata

**Key Files:**
- `services/time/worldtime.go`
- `services/time/models.go`
- `services/time/worldtime_test.go`

### 4. REST Countries Service

**Source:** [REST Countries API](https://restcountries.com/)

**Functionality:**
- Fetches country metadata (flags, capitals, languages)
- Parses structured country information
- Stores country reference data

**Key Files:**
- `services/country/country.go`
- `services/country/models.go`
- `services/country/country_test.go`

---

## API Keys

### Obtaining API Keys

**WeatherAPI.com**
1. Visit [weatherapi.com](https://www.weatherapi.com/)
2. Sign up for a free account
3. Copy your API key from the dashboard
4. Paste into `.env` as `WEATHER_API_KEY`

**Other APIs**
- OpenAQ, World Time, and REST Countries do **not require authentication**
- You can use them immediately after configuration

### Rotating Keys

To update API keys:
1. Edit `.env` with new key values
2. Restart the application
3. Configuration is reloaded on startup

---

## Troubleshooting

### MongoDB Connection Errors

**Error:** `failed to connect to MongoDB`

**Solution:**
```bash
# Verify MongoDB is running
docker ps | grep mongo

# Restart MongoDB
docker-compose restart mongo

# Check connection string in .env
echo $MONGODB_URI
```

### API Request Failures

**Error:** `failed to fetch data from API`

**Solution:**
- Verify API keys in `.env`
- Check API rate limits (particularly WeatherAPI free tier)
- Inspect logs for detailed error messages

### Docker Build Failures

**Error:** `failed to build image`

**Solution:**
```bash
# Clean up Docker
docker system prune

# Rebuild
docker-compose build --no-cache
```

### Test Failures (Docker Required)

**Error:** `Cannot connect to the Docker daemon`

**Solution:**
```bash
# Start Docker daemon (on Linux)
sudo systemctl start docker

# Verify Docker is running
docker ps

# Rerun tests
go test ./...
```

---

## Performance Considerations

- **Worker Pool Size:** Configurable in `internal/workpool/workpool.go` (default: 5 workers)
- **Channel Buffers:** 100-item buffers for non-blocking sends
- **Database Batch Inserts:** Uses MongoDB InsertMany for efficiency
- **Concurrent Testing:** All tests run in parallel using Go's native test runner

---

## Contributing

1. Create a feature branch: `git checkout -b feature/your-feature`
2. Write tests for new functionality
3. Ensure all tests pass: `go test ./...`
4. Commit with clear messages: `git commit -m "Add feature: description"`
5. Push and open a pull request

---

## License

This project is open source. See LICENSE file for details.

---

## Support

For issues, questions, or contributions, please open an issue on GitHub.

**Repository:** [go-api-parser-mono](https://github.com/AbdulWasayUl/go-api-parser-mono)

---

## Changelog

### v1.0.0 (Current)

- ✅ Complete service implementations (Weather, AQI, World Time, Countries)
- ✅ Worker pool for concurrent processing
- ✅ Cron-based scheduler
- ✅ Comprehensive test suite (85%+ coverage)
- ✅ Docker and Docker Compose support
- ✅ MongoDB integration with migrations

