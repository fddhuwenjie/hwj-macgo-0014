package domain

import (
	"encoding/json"
	"time"
)

type Event struct {
	Type      string
	EntityID  string
	Payload   json.RawMessage
	Timestamp time.Time
}

type SnapshotMetadata struct {
	Version   int64
	CreatedAt time.Time
	Checksum  string
}
