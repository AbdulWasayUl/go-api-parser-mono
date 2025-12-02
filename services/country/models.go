package country

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FetchParam struct {
	CountryCode string `bson:"country_code"`
	CountryName string `bson:"country_name"`
}

type RestCountriesAPIResponse []struct {
	Name struct {
		Common   string `json:"common"`
		Official string `json:"official"`
	} `json:"name"`
	CCA2        string `json:"cca2"`
	Independent bool   `json:"independent"`
	UnMember    bool   `json:"unMember"`
	Currencies  map[string]struct {
		Name   string `json:"name"`
		Symbol string `json:"symbol"`
	} `json:"currencies"`
	Population int      `json:"population"`
	Region     string   `json:"region"`
	Subregion  string   `json:"subregion"`
	Area       float64  `json:"area"`
	Capital    []string `json:"capital"`
}

type CountryData struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	CountryCode  string             `bson:"country_code"`
	OfficialName string             `bson:"official_name"`
	CommonName   string             `bson:"common_name"`
	Independent  bool               `bson:"independent"`
	UnMember     bool               `bson:"un_member"`
	Capital      string             `bson:"capital"`
	Region       string             `bson:"region"`
	Subregion    string             `bson:"subregion"`
	Currency     string             `bson:"currency"`
	CurrencySym  string             `bson:"currency_symbol"`
	Population   int                `bson:"population"`
	Area         float64            `bson:"area_sqkm"`
	FetchedAt    time.Time          `bson:"fetched_at"`
}
