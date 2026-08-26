package repository

import (
	"encoding/json"

	"acousticverdictworkbench/internal/domain"
)

func cloneBatch(batch *domain.ReviewBatch) (*domain.ReviewBatch, error) {
	data, err := json.Marshal(batch)
	if err != nil {
		return nil, err
	}
	var copy domain.ReviewBatch
	if err := json.Unmarshal(data, &copy); err != nil {
		return nil, err
	}
	return &copy, nil
}
