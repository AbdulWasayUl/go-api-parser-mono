package channels

import (
	"sync"

	"github.com/AbdulWasayUl/go-api-parser-mono/models"
)

type Channels struct {
	DataRequest chan models.DataRequest
	WG          *sync.WaitGroup
}

func New() *Channels {
	const bufferSize = 100
	return &Channels{
		DataRequest: make(chan models.DataRequest, bufferSize),
		WG:          &sync.WaitGroup{},
	}
}
