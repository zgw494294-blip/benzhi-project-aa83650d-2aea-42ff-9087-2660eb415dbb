package view_cache_identity_leak_test

import (
	"context"
	"testing"
	"time"

	"acousticverdictworkbench/internal/application"
	"acousticverdictworkbench/internal/domain"
	"acousticverdictworkbench/internal/repository"
)

func TestBatchViewCacheMustRespectViewerAndCommittedVersion(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	batch, err := domain.NewReviewBatch("batch-cache", "缓存身份隔离", "SITE-CACHE", now, now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := batch.ConfigureScope(batch.Title, batch.SiteCode, batch.RecordingStart, batch.RecordingEnd, []string{"BIRD_A"}, now); err != nil {
		t.Fatal(err)
	}
	if err := batch.AddClip(domain.AudioClip{ID: "clip-cache", SourceName: "cache.wav", CapturedAt: now.Add(time.Minute), DurationMs: 2000, ContentSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Sequence: 1}, now); err != nil {
		t.Fatal(err)
	}
	if err := batch.Freeze(now); err != nil {
		t.Fatal(err)
	}
	firstDraft := []domain.CandidateEvent{{ID: "event-a", SpeciesCode: "BIRD_A", StartMs: 100, EndMs: 400, Confidence: domain.ConfidenceHigh, EvidenceNote: "标注员甲的未提交证据"}}
	if err := batch.SaveDraft("submission-a", "clip-cache", "annotator-a", 1, firstDraft, "", now); err != nil {
		t.Fatal(err)
	}
	seeded, err := store.Commit(ctx, repository.CommitRequest{Batch: batch, ExpectedVersion: 0, Operation: "test.seed", Events: batch.DrainEvents(), CommittedAt: now})
	if err != nil {
		t.Fatal(err)
	}

	sequence := 0
	service := application.NewConfigured(store, func() time.Time { return now }, func(prefix string) string {
		sequence++
		return prefix + "-generated"
	})
	ownerView, err := service.GetBatch(ctx, seeded.Batch.ID, "annotator-a", "annotator")
	if err != nil {
		t.Fatal(err)
	}
	if len(ownerView.Submissions) != 1 {
		t.Fatalf("测试前提无效：草稿所有者未看到自己的草稿：%+v", ownerView.Submissions)
	}

	otherView, err := service.GetBatch(ctx, seeded.Batch.ID, "annotator-b", "annotator")
	if err != nil {
		t.Fatal(err)
	}
	if len(otherView.Submissions) != 0 {
		t.Fatalf("身份过滤后的缓存跨标注员复用，annotator-b 看到了 annotator-a 的未提交草稿：%+v", otherView.Submissions)
	}

	updatedDraft := []domain.CandidateEvent{{ID: "event-a", SpeciesCode: "BIRD_A", StartMs: 500, EndMs: 900, Confidence: domain.ConfidenceMedium, EvidenceNote: "更新后的未提交证据"}}
	updated, err := service.SaveDraft(ctx, application.DraftCommand{Metadata: application.Metadata{ActorID: "annotator-a", Role: "annotator", ExpectedVersion: seeded.Batch.Version, IdempotencyKey: "draft-update"}, BatchID: seeded.Batch.ID, ClipID: "clip-cache", SubmissionID: "submission-a", AnnotatorID: "annotator-a", Round: 1, Events: updatedDraft})
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := service.GetBatch(ctx, seeded.Batch.ID, "annotator-a", "annotator")
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Batch.Version != updated.Version || refreshed.Submissions[0].Events[0].StartMs != 500 {
		t.Fatalf("提交成功后查询仍复用旧批次版本：cached=%d committed=%d event=%+v", refreshed.Batch.Version, updated.Version, refreshed.Submissions[0].Events[0])
	}
}
