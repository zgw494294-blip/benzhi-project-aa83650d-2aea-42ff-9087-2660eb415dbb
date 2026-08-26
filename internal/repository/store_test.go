package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"acousticverdictworkbench/internal/domain"
)

func TestStorePersistsExactIdempotentResultAndRecovers(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	batch, _ := domain.NewReviewBatch("batch", "测试批次", "SITE", now, now.Add(time.Hour), now)
	events := batch.DrainEvents()
	result, err := store.Commit(context.Background(), CommitRequest{Batch: batch, ExpectedVersion: 0, Operation: "create", IdempotencyKey: "create:key", Events: events, CommittedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	createdVersion := result.Batch.Version
	if err := batch.ConfigureScope(batch.Title, batch.SiteCode, now, now.Add(time.Hour), []string{"BIRD_A"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Commit(context.Background(), CommitRequest{Batch: batch, ExpectedVersion: createdVersion, Operation: "scope", Events: batch.DrainEvents(), CommittedAt: now}); err != nil {
		t.Fatal(err)
	}
	idempotent, ok := store.Idempotent(context.Background(), "create:key")
	if !ok || idempotent.Batch.Version != createdVersion {
		t.Fatalf("幂等结果不是原始版本：%+v", idempotent)
	}
	_ = store.Close()
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restored, err := reopened.Get(context.Background(), "batch")
	if err != nil {
		t.Fatal(err)
	}
	if restored.Version != batch.Version || len(restored.AllowedSpeciesCodes) != 1 {
		t.Fatalf("恢复投影错误：%+v", restored)
	}
}

func TestStoreRejectsBrokenAuditChain(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "audit.jsonl"), []byte("{\"schemaVersion\":1,\"sequence\":2}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("损坏审计链未被拒绝")
	}
}
