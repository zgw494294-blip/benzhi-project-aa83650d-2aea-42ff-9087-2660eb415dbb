package retryauditgap_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"acousticverdictworkbench/internal/domain"
	"acousticverdictworkbench/internal/repository"
)

func TestRetryAfterPartialCommitMustRemainAudited(t *testing.T) {
	dir := t.TempDir()
	store, err := repository.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	first, err := domain.NewReviewBatch("first", "first", "SITE", now, now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	// 让审计追加成功、快照 Rename 失败，模拟一次确定性的部分持久化故障。
	blocker := filepath.Join(dir, "snapshots", "first.json")
	if err := os.Mkdir(blocker, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err = store.Commit(context.Background(), repository.CommitRequest{
		Batch: first, ExpectedVersion: 0, Operation: "batch.create",
		Events: first.DrainEvents(), CommittedAt: now,
	})
	if err == nil {
		t.Fatal("预期第一次提交在写快照时失败")
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}

	second, err := domain.NewReviewBatch("second", "second", "SITE", now, now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Commit(context.Background(), repository.CommitRequest{
		Batch: second, ExpectedVersion: 0, Operation: "batch.create",
		Events: second.DrainEvents(), CommittedAt: now,
	})
	if err != nil {
		t.Fatalf("重试提交失败：%v", err)
	}
	audit, err := os.ReadFile(filepath.Join(dir, "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(audit), `"batchId":"second"`) {
		t.Fatalf("TestRetryAfterPartialCommitMustRemainAudited: 第二次提交已返回成功，但审计日志没有 second 批次：%s", audit)
	}
}
