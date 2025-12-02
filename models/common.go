package models

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

type DataRequest struct {
	ID        string
	Service   string
	FetchFunc func(ctx context.Context, id string) ([]byte, error)
	ParseFunc func([]byte) (interface{}, error)
	StoreFunc func(ctx context.Context, data interface{}) error
}

// type Service interface {
// 	FetchData(ctx context.Context, id string) ([]byte, error)
// 	ParseData(data []byte) (interface{}, error)
// 	StoreData(ctx context.Context, data interface{}) error
// }

type RateLimitSettings struct {
	MaxRequests int
	PerDuration time.Duration
}

type Migration struct {
	Name string
	Func func(ctx context.Context, client *mongo.Client) error
}
