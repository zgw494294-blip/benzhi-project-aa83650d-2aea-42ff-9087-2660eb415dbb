package repository

import (
	"encoding/json"
	"time"
)

const schemaVersion = 1

type snapshotEnvelope struct {
	SchemaVersion int       `json:"schemaVersion"`
	SavedAt       time.Time `json:"savedAt"`
	Batch         jsonRaw   `json:"batch"`
}

type jsonRaw = json.RawMessage

type auditRecord struct {
	SchemaVersion int       `json:"schemaVersion"`
	Sequence      uint64    `json:"sequence"`
	PreviousHash  string    `json:"previousHash"`
	Hash          string    `json:"hash"`
	BatchID       string    `json:"batchId"`
	BatchVersion  uint64    `json:"batchVersion"`
	Operation     string    `json:"operation"`
	OccurredAt    time.Time `json:"occurredAt"`
	EventType     string    `json:"eventType"`
	EventDetails  any       `json:"eventDetails,omitempty"`
}

type auditPayload struct {
	SchemaVersion int       `json:"schemaVersion"`
	Sequence      uint64    `json:"sequence"`
	PreviousHash  string    `json:"previousHash"`
	BatchID       string    `json:"batchId"`
	BatchVersion  uint64    `json:"batchVersion"`
	Operation     string    `json:"operation"`
	OccurredAt    time.Time `json:"occurredAt"`
	EventType     string    `json:"eventType"`
	EventDetails  any       `json:"eventDetails,omitempty"`
}

type idempotencyFile struct {
	SchemaVersion int                          `json:"schemaVersion"`
	Results       map[string]idempotencyRecord `json:"results"`
}

type idempotencyRecord struct {
	BatchID string          `json:"batchId"`
	Version uint64          `json:"version"`
	Batch   json.RawMessage `json:"batch"`
}
