package persistence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"phonemereleasedesk/internal/domain"
)

type CommitResult struct {
	Batch      *domain.ReleaseBatch `json:"batch"`
	StatusCode int                  `json:"statusCode"`
}

type Repository interface {
	Get(context.Context, string) (*domain.ReleaseBatch, error)
	List(context.Context) ([]*domain.ReleaseBatch, error)
	Commit(context.Context, *domain.ReleaseBatch, uint64, string, string, time.Time) (*domain.ReleaseBatch, error)
	Idempotent(context.Context, string) (CommitResult, bool)
	SaveIdempotent(context.Context, string, CommitResult) error
	Close() error
}

type FileRepository struct {
	dir         string
	ledgerPath  string
	idemPath    string
	mu          sync.RWMutex
	batches     map[string]*domain.ReleaseBatch
	idempotency map[string]CommitResult
	sequence    uint64
	lastHash    string
	closed      bool
}

func Open(dir string) (*FileRepository, error) {
	if dir == "" {
		return nil, fmt.Errorf("数据目录不能为空")
	}
	if err := os.MkdirAll(filepath.Join(dir, "snapshots"), 0o755); err != nil {
		return nil, err
	}
	repo := &FileRepository{dir: dir, ledgerPath: filepath.Join(dir, "events.jsonl"), idemPath: filepath.Join(dir, "idempotency.json"), batches: map[string]*domain.ReleaseBatch{}, idempotency: map[string]CommitResult{}}
	if err := repo.recoverLedger(); err != nil {
		return nil, err
	}
	if err := repo.loadIdempotency(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *FileRepository) Get(_ context.Context, id string) (*domain.ReleaseBatch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	batch, exists := r.batches[id]
	if !exists {
		return nil, domain.ErrNotFound
	}
	return batch.Clone(), nil
}

func (r *FileRepository) List(_ context.Context) ([]*domain.ReleaseBatch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.ReleaseBatch, 0, len(r.batches))
	for _, batch := range r.batches {
		result = append(result, batch.Clone())
	}
	return result, nil
}

func (r *FileRepository) Commit(ctx context.Context, batch *domain.ReleaseBatch, expected uint64, eventType, idempotencyKey string, now time.Time) (*domain.ReleaseBatch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, errors.New("仓储已关闭")
	}
	current, exists := r.batches[batch.ID]
	if exists {
		if current.Version != expected {
			return nil, domain.ErrVersionConflict
		}
	} else if expected != 0 {
		return nil, domain.ErrVersionConflict
	}
	if exists && batch.Version <= current.Version {
		return nil, domain.Invalid("version", "提交版本必须递增")
	}
	if !exists && batch.Version == 0 {
		return nil, domain.Invalid("version", "初始版本必须大于零")
	}
	event, err := domain.SnapshotEvent(batch, eventType, now)
	if err != nil {
		return nil, err
	}
	record, err := makeRecord(r.sequence+1, r.lastHash, event)
	if err != nil {
		return nil, err
	}
	if err := r.appendRecord(record); err != nil {
		return nil, err
	}
	r.sequence, r.lastHash = record.Sequence, record.Hash
	r.batches[batch.ID] = batch.Clone()
	if err := r.writeSnapshot(batch); err != nil {
		return nil, err
	}
	if idempotencyKey != "" {
		r.idempotency[idempotencyKey] = CommitResult{Batch: batch.Clone(), StatusCode: 200}
		if err := r.writeIdempotency(); err != nil {
			return nil, err
		}
	}
	return batch.Clone(), nil
}

func (r *FileRepository) Idempotent(_ context.Context, key string) (CommitResult, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result, ok := r.idempotency[key]
	if ok && result.Batch != nil {
		result.Batch = result.Batch.Clone()
	}
	return result, ok
}

func (r *FileRepository) SaveIdempotent(_ context.Context, key string, result CommitResult) error {
	if key == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if result.Batch != nil {
		result.Batch = result.Batch.Clone()
	}
	r.idempotency[key] = result
	return r.writeIdempotency()
}

func (r *FileRepository) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}
