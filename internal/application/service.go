package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"acousticverdictworkbench/internal/domain"
	"acousticverdictworkbench/internal/quality"
	"acousticverdictworkbench/internal/repository"
)

type Clock func() time.Time
type IDGenerator func(string) string

type Service struct {
	repo    repository.Repository
	now     Clock
	ids     IDGenerator
	matcher quality.Matcher
	checker quality.Checker
}

func New(repo repository.Repository) *Service {
	return &Service{repo: repo, now: time.Now, ids: randomID, matcher: quality.NewMatcher(), checker: quality.NewChecker()}
}

func NewConfigured(repo repository.Repository, now Clock, ids IDGenerator) *Service {
	return &Service{repo: repo, now: now, ids: ids, matcher: quality.NewMatcher(), checker: quality.NewChecker()}
}

func randomID(prefix string) string {
	buffer := make([]byte, 10)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(buffer)
}

func authorize(meta Metadata, roles ...string) error {
	if strings.TrimSpace(meta.ActorID) == "" {
		return domain.Invalid("actorId", "操作者 ID 不能为空")
	}
	for _, role := range roles {
		if meta.Role == role {
			return nil
		}
	}
	return domain.ErrForbidden
}

func (s *Service) existing(ctx context.Context, operation string, meta Metadata) (*domain.ReviewBatch, error, bool) {
	if meta.IdempotencyKey == "" {
		return nil, nil, false
	}
	result, ok := s.repo.Idempotent(ctx, operation+":"+meta.IdempotencyKey)
	if !ok {
		return nil, nil, false
	}
	if !sameActor(result.ActorID, result.Role, meta) {
		return nil, domain.ErrForbidden, true
	}
	return result.Batch, nil, true
}

// sameActor 判断当前请求是否与持久化幂等结果原始授权者一致。
// 持久化记录缺失 actor 信息（例如旧版本写入）时按 owner 字段非空且完全匹配判定，
// 避免历史记录因迁移而退化为可被任意身份重放。
func sameActor(recordedActor, recordedRole string, meta Metadata) bool {
	if recordedActor == "" && recordedRole == "" {
		return false
	}
	return recordedActor == meta.ActorID && recordedRole == meta.Role
}

func (s *Service) commit(ctx context.Context, batch *domain.ReviewBatch, expected uint64, operation, key string, meta Metadata) (*domain.ReviewBatch, error) {
	events := batch.DrainEvents()
	persistenceKey := ""
	if key != "" {
		persistenceKey = operation + ":" + key
	}
	result, err := s.repo.Commit(ctx, repository.CommitRequest{Batch: batch, ExpectedVersion: expected, Operation: operation, IdempotencyKey: persistenceKey, ActorID: meta.ActorID, Role: meta.Role, Events: events, CommittedAt: s.now()})
	if err != nil {
		return nil, err
	}
	return result.Batch, nil
}

func (s *Service) GetBatch(ctx context.Context, id, viewer, role string) (*BatchView, error) {
	batch, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	tasks := batch.VisibleReannotationTasks(viewer, role)
	progress := ReannotationProgress{}
	for _, task := range tasks {
		switch task.Status {
		case domain.ReannotationPending:
			progress.Pending++
		case domain.ReannotationInProgress:
			progress.InProgress++
		case domain.ReannotationClosed:
			progress.Closed++
		}
	}
	return &BatchView{Batch: batch, Submissions: batch.VisibleSubmissions(viewer, role), OpenQueue: batch.OpenDisputes(), ReannotationTasks: tasks, ReannotationProgress: progress}, nil
}

func (s *Service) ListBatches(ctx context.Context) ([]*domain.ReviewBatch, error) {
	return s.repo.List(ctx)
}
