package repository

import (
	"context"
	"time"

	"acousticverdictworkbench/internal/domain"
)

type CommitRequest struct {
	Batch           *domain.ReviewBatch
	ExpectedVersion uint64
	Operation       string
	IdempotencyKey  string
	ActorID         string
	Role            string
	Events          []domain.DomainEvent
	CommittedAt     time.Time
}

type CommitResult struct {
	Batch   *domain.ReviewBatch `json:"batch"`
	ActorID string             `json:"actorId,omitempty"`
	Role    string             `json:"role,omitempty"`
}

type Repository interface {
	Get(context.Context, string) (*domain.ReviewBatch, error)
	List(context.Context) ([]*domain.ReviewBatch, error)
	Commit(context.Context, CommitRequest) (*CommitResult, error)
	Idempotent(context.Context, string) (*CommitResult, bool)
	Close() error
}
