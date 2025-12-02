package aqi

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FetchParam struct {
	CountryId   string `bson:"country_id"`
	CountryCode string `bson:"country_code"`
	CountryName string `bson:"country_name"`
}

type AQIAPIResponse struct {
	Results []struct {
		Id         int    `json:"id"`
		Code       string `json:"code"`
		Name       string `json:"name"`
		Parameters []struct {
			Parameter string `json:"parameter"`
			Units     string `json:"units"`
		} `json:"parameters"`
	} `json:"results"`
}

type AQIData struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	CountryID   int                `bson:"country_id"`
	CountryName string             `bson:"country_name"`
	Parameters  []struct {
		Name  string `bson:"name"`
		Units string `bson:"units"`
	} `bson:"parameters"`
	FetchedAt time.Time `bson:"fetched_at"`
}
