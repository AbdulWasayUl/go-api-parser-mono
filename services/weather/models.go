package weather

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FetchParam struct {
	City    string  `bson:"city"`
	Country string  `bson:"country"`
	Lat     float64 `bson:"lat"`
	Lon     float64 `bson:"lon"`
}

type WeatherAPIResponse struct {
	Location struct {
		Name    string `json:"name"`
		Country string `json:"country"`
		Region  string `json:"region"`
		TzID    string `json:"tz_id"`
	} `json:"location"`
	Current struct {
		LastUpdated string  `json:"last_updated"`
		TempC       float64 `json:"temp_c"`
		TempF       float64 `json:"temp_f"`
		Condition   struct {
			Text string `json:"text"`
		} `json:"condition"`
		WindKPH    float64 `json:"wind_kph"`
		WindMPH    float64 `json:"wind_mph"`
		WindDir    string  `json:"wind_dir"`
		WindDegree int     `json:"wind_degree"`
		PressureMB float64 `json:"pressure_mb"`
		PressureIN float64 `json:"pressure_in"`
		PrecipMM   float64 `json:"precip_mm"`
		PrecipIN   float64 `json:"precip_in"`
		Humidity   int     `json:"humidity"`
		Cloud      int     `json:"cloud"`
	} `json:"current"`
}

type WeatherData struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	City         string             `bson:"city"`
	Country      string             `bson:"country"`
	Region       string             `bson:"region"`
	TzID         string             `bson:"tz_id"`
	TemperatureF float64            `bson:"temperature_f"`
	TemperatureC float64            `bson:"temperature_c"`
	Condition    string             `bson:"condition"`
	WindKPH      float64            `bson:"wind_kph"`
	WindMPH      float64            `bson:"wind_mph"`
	WindDir      string             `bson:"wind_dir"`
	WindDegree   int                `bson:"wind_degree"`
	PressureMB   float64            `bson:"pressure_mb"`
	PressureIN   float64            `bson:"pressure_in"`
	PrecipMM     float64            `bson:"precip_mm"`
	PrecipIN     float64            `bson:"precip_in"`
	Humidity     int                `bson:"humidity"`
	Cloud        int                `bson:"cloud"`
	FetchedAt    time.Time          `bson:"fetched_at"`
}
