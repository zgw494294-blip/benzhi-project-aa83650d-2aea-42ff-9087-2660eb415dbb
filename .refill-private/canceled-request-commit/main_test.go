package canceledrequestcommit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"acousticverdictworkbench/internal/application"
	"acousticverdictworkbench/internal/domain"
	"acousticverdictworkbench/internal/repository"
	"acousticverdictworkbench/internal/webui"
)

type gatedRepository struct {
	inner       repository.Repository
	getEntered  chan struct{}
	continueGet chan struct{}
	committed   bool
}

func (r *gatedRepository) Get(ctx context.Context, id string) (*domain.ReviewBatch, error) {
	close(r.getEntered)
	<-r.continueGet
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.inner.Get(ctx, id)
}

func (r *gatedRepository) List(ctx context.Context) ([]*domain.ReviewBatch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.inner.List(ctx)
}

func (r *gatedRepository) Commit(ctx context.Context, request repository.CommitRequest) (*repository.CommitResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result, err := r.inner.Commit(ctx, request)
	if err == nil {
		r.committed = true
	}
	return result, err
}

func (r *gatedRepository) Idempotent(ctx context.Context, key string) (*repository.CommitResult, bool) {
	if ctx.Err() != nil {
		return nil, false
	}
	return r.inner.Idempotent(ctx, key)
}

func (r *gatedRepository) Close() error { return r.inner.Close() }

func TestCanceledRequestMustNotCommitScopeChange(t *testing.T) {
	now := time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)
	batch, err := domain.NewReviewBatch("batch-cancel", "原始批次", "SITE-A", now, now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Commit(context.Background(), repository.CommitRequest{Batch: batch, ExpectedVersion: 0, Operation: "test.seed", Events: batch.DrainEvents(), CommittedAt: now}); err != nil {
		t.Fatal(err)
	}
	repo := &gatedRepository{
		inner:       store,
		getEntered:  make(chan struct{}),
		continueGet: make(chan struct{}),
	}
	service := application.NewConfigured(repo, func() time.Time { return now }, func(prefix string) string { return prefix + "-fixed" })
	handler := webui.New(service).Handler()
	payload, err := json.Marshal(map[string]any{
		"actorId":             "manager-a",
		"role":                "manager",
		"expectedVersion":     batch.Version,
		"title":               "取消后不应保存",
		"siteCode":            "SITE-B",
		"recordingStart":      now,
		"recordingEnd":        now.Add(2 * time.Hour),
		"allowedSpeciesCodes": []string{"BIRD_B"},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodPut, "/api/batches/"+batch.ID+"/scope", bytes.NewReader(payload)).WithContext(ctx)
	request.Header.Set("Idempotency-Key", "cancel-scope")
	response := httptest.NewRecorder()
	finished := make(chan struct{})
	go func() {
		handler.ServeHTTP(response, request)
		close(finished)
	}()

	<-repo.getEntered
	cancel()
	close(repo.continueGet)
	<-finished

	stored, err := store.Get(context.Background(), batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if repo.committed || stored.Version != batch.Version {
		t.Fatalf("已取消的 HTTP 请求仍提交了 scope 变更，storedVersion=%d status=%d body=%s", stored.Version, response.Code, response.Body.String())
	}
}
