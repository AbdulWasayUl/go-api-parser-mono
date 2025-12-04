package channels_test

import (
	"testing"

	"github.com/AbdulWasayUl/go-api-parser-mono/internal/channels"
	"github.com/AbdulWasayUl/go-api-parser-mono/models"
)

func TestChannels_Table(t *testing.T) {
	tests := []struct {
		name     string
		inputID  string
		expected string
	}{
		{"SingleMessage", "123", "123"},
		{"AnotherMessage", "abc", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := channels.New()

			ch.DataRequest <- models.DataRequest{ID: tt.inputID}

			got := (<-ch.DataRequest).ID

			if got != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}
