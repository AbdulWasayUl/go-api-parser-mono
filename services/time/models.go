package time

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FetchParam struct {
	CountryCode string `bson:"country_code"`
	Timezone    string `bson:"timezone"`
}

type WorldTimeAPIResponse struct {
	UTCOffset    string    `json:"utc_offset"`
	Timezone     string    `json:"timezone"`
	DayOfWeek    int       `json:"day_of_week"`
	Datetime     time.Time `json:"datetime"`
	UTCDatetime  time.Time `json:"utc_datetime"`
	WeekNumber   int       `json:"week_number"`
	DST          bool      `json:"dst"`
	Abbreviation string    `json:"abbreviation"`
}

type WorldTimeData struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	Timezone     string             `bson:"timezone"`
	UTCOffset    string             `bson:"utc_offset"`
	CurrentTime  time.Time          `bson:"current_time"`
	DayOfWeek    int                `bson:"day_of_week"`
	WeekNumber   int                `bson:"week_number"`
	UTCDatetime  time.Time          `bson:"utc_datetime"`
	IsDST        bool               `bson:"is_dst"`
	Abbreviation string             `bson:"abbreviation"`
	FetchedAt    time.Time          `bson:"fetched_at"`
}
