package repository_read_lock_gap_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"acousticverdictworkbench/internal/domain"
	"acousticverdictworkbench/internal/repository"
)

func TestConcurrentProjectionReadsMustSynchronizeWithCommit(t *testing.T) {
	ctx := context.Background()
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	batch, err := domain.NewReviewBatch("batch-race", "并发投影测试", "SITE-RACE", now, now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.Commit(ctx, repository.CommitRequest{
		Batch:           batch,
		ExpectedVersion: 0,
		Operation:       "batch.create",
		IdempotencyKey:  "batch.create:seed",
		Events:          batch.DrainEvents(),
		CommittedAt:     now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := batch.ConfigureScope(batch.Title, batch.SiteCode, now, now.Add(time.Hour), []string{"BIRD_A"}, now); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(2)
	var finished sync.WaitGroup
	finished.Add(2)

	go func() {
		defer finished.Done()
		ready.Done()
		<-start
		if _, err := store.Commit(ctx, repository.CommitRequest{
			Batch:           batch,
			ExpectedVersion: created.Batch.Version,
			Operation:       "batch.configure",
			IdempotencyKey:  "batch.configure:update",
			Events:          batch.DrainEvents(),
			CommittedAt:     now,
		}); err != nil {
			t.Errorf("提交失败：%v", err)
		}
	}()

	go func() {
		defer finished.Done()
		ready.Done()
		<-start
		if _, err := store.Get(ctx, batch.ID); err != nil {
			t.Errorf("Get 失败：%v", err)
		}
		if _, err := store.List(ctx); err != nil {
			t.Errorf("List 失败：%v", err)
		}
		if _, ok := store.Idempotent(ctx, "batch.create:seed"); !ok {
			t.Error("Idempotent 未返回已提交结果")
		}
	}()

	ready.Wait()
	close(start)
	finished.Wait()
}
