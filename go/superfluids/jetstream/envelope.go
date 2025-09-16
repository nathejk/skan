package jetstream

import (
	"encoding/json"
	"time"
)

type EventID string
type Version int

// struct 'jetstreamMessage' represents a message on the jetstream
type jetstreamMessage struct {
	EventID       EventID         `json:"eventId"`
	CorrelationID EventID         `json:"correlationId"`
	CausationID   EventID         `json:"causationId"`
	Version       Version         `json:"version,omitempty"`
	Time          time.Time       `json:"time"`
	Body          json.RawMessage `json:"body"`
	Meta          json.RawMessage `json:"meta"`
}
