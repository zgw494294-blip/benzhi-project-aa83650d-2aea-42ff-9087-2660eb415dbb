package domain

import (
	"encoding/json"
	"time"
)

type Event struct {
	Type      string          `json:"type"`
	BatchID   string          `json:"batchId"`
	Version   uint64          `json:"version"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type BatchSnapshot struct {
	SchemaVersion int           `json:"schemaVersion"`
	LastSequence  uint64        `json:"lastSequence"`
	LastHash      string        `json:"lastHash"`
	Batch         *ReleaseBatch `json:"batch"`
}

func SnapshotEvent(batch *ReleaseBatch, eventType string, now time.Time) (Event, error) {
	payload, err := json.Marshal(batch)
	if err != nil {
		return Event{}, err
	}
	return Event{Type: eventType, BatchID: batch.ID, Version: batch.Version, Timestamp: now.UTC(), Payload: payload}, nil
}

func Replay(events []Event) (*ReleaseBatch, error) {
	var current *ReleaseBatch
	var lastVersion uint64
	for _, event := range events {
		if event.Version <= lastVersion {
			return nil, Invalid("event.version", "事件版本未递增")
		}
		var next ReleaseBatch
		if err := json.Unmarshal(event.Payload, &next); err != nil {
			return nil, err
		}
		if next.ID != event.BatchID || next.Version != event.Version {
			return nil, Invalid("event", "事件负载与信封不一致")
		}
		current = &next
		lastVersion = event.Version
	}
	if current == nil {
		return nil, ErrNotFound
	}
	return current, nil
}
