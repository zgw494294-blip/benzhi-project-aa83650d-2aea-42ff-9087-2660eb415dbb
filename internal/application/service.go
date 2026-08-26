package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"phonemereleasedesk/internal/domain"
	"phonemereleasedesk/internal/persistence"
)

type Clock func() time.Time
type IDGenerator func(string) string

type Service struct {
	repo     persistence.Repository
	now      Clock
	ids      IDGenerator
	commitMu sync.Mutex
}

func New(repo persistence.Repository) *Service {
	return &Service{repo: repo, now: time.Now, ids: randomID}
}

func NewConfigured(repo persistence.Repository, now Clock, ids IDGenerator) *Service {
	return &Service{repo: repo, now: now, ids: ids}
}

func randomID(prefix string) string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(buffer)
}

func (s *Service) GetBatch(ctx context.Context, id string) (*domain.ReleaseBatch, error) {
	return s.repo.Get(ctx, id)
}
func (s *Service) ListBatches(ctx context.Context) ([]*domain.ReleaseBatch, error) {
	return s.repo.List(ctx)
}

func authorize(meta Metadata, roles ...string) error {
	for _, role := range roles {
		if meta.Role == role {
			return nil
		}
	}
	return domain.ErrForbidden
}

func (s *Service) idempotent(ctx context.Context, operation, key string) (*domain.ReleaseBatch, bool) {
	if key == "" {
		return nil, false
	}
	result, exists := s.repo.Idempotent(ctx, operation+":"+key)
	return result.Batch, exists
}

func (s *Service) commit(ctx context.Context, batch *domain.ReleaseBatch, expected uint64, operation, key string) (*domain.ReleaseBatch, error) {
	s.commitMu.Lock()
	defer s.commitMu.Unlock()
	persistenceKey := ""
	if key != "" {
		persistenceKey = operation + ":" + key
		if result, exists := s.repo.Idempotent(ctx, persistenceKey); exists {
			return result.Batch, nil
		}
	}
	return s.repo.Commit(ctx, batch, expected, operation, persistenceKey, s.now())
}
