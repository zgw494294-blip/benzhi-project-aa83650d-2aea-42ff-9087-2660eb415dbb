package repository

import (
	"fmt"

	"acousticverdictworkbench/internal/domain"
)

func cloneBatch(batch *domain.ReviewBatch) (*domain.ReviewBatch, error) {
	if batch == nil {
		return nil, fmt.Errorf("不能复制空批次")
	}
	copy := *batch
	return &copy, nil
}
