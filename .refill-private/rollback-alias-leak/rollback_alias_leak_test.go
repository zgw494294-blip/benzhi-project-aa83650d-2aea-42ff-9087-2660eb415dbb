package rollback_alias_leak_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"acousticverdictworkbench/internal/application"
	"acousticverdictworkbench/internal/domain"
	"acousticverdictworkbench/internal/repository"
)

func TestVersionConflictMustNotLeakDraftMutation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	batch, err := domain.NewReviewBatch("batch-alias", "别名隔离复现", "SITE-A", now, now.Add(time.Hour), now)
	if err != nil {
		t.Fatalf("new batch: %v", err)
	}
	if err := batch.ConfigureScope(batch.Title, batch.SiteCode, batch.RecordingStart, batch.RecordingEnd, []string{"BIRD_A", "BIRD_B"}, now); err != nil {
		t.Fatalf("configure scope: %v", err)
	}
	if err := batch.AddClip(domain.AudioClip{ID: "clip-1", SourceName: "clip.wav", CapturedAt: now.Add(time.Minute), DurationMs: 2_000, ContentSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Sequence: 1}, now); err != nil {
		t.Fatalf("add clip: %v", err)
	}
	if err := batch.Freeze(now); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	original := domain.CandidateEvent{ID: "event-original", SpeciesCode: "BIRD_A", StartMs: 100, EndMs: 400, Confidence: domain.ConfidenceHigh, EvidenceNote: "原始草稿"}
	if err := batch.SaveDraft("submission-1", "clip-1", "annotator-a", 1, []domain.CandidateEvent{original}, "", now); err != nil {
		t.Fatalf("seed draft: %v", err)
	}
	events := batch.DrainEvents()
	if _, err := store.Commit(ctx, repository.CommitRequest{Batch: batch, ExpectedVersion: 0, Operation: "test.seed", Events: events, CommittedAt: now}); err != nil {
		t.Fatalf("commit fixture: %v", err)
	}

	service := application.NewConfigured(store, func() time.Time { return now.Add(time.Minute) }, func(prefix string) string { return prefix + "-fixed" })
	replacement := domain.CandidateEvent{ID: "event-rejected", SpeciesCode: "BIRD_B", StartMs: 500, EndMs: 900, Confidence: domain.ConfidenceMedium, EvidenceNote: "本次写入应被拒绝"}
	_, err = service.SaveDraft(ctx, application.DraftCommand{
		Metadata:     application.Metadata{ActorID: "annotator-a", Role: "annotator", ExpectedVersion: batch.Version - 1},
		BatchID:      batch.ID,
		ClipID:       "clip-1",
		SubmissionID: "submission-ignored",
		AnnotatorID:  "annotator-a",
		Round:        1,
		Events:       []domain.CandidateEvent{replacement},
	})
	if !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}

	stored, err := store.Get(ctx, batch.ID)
	if err != nil {
		t.Fatalf("read after rejected commit: %v", err)
	}
	if len(stored.Submissions) != 1 || len(stored.Submissions[0].Events) != 1 {
		t.Fatalf("unexpected stored fixture shape: %+v", stored.Submissions)
	}
	got := stored.Submissions[0].Events[0]
	if got.ID != original.ID || got.SpeciesCode != original.SpeciesCode || got.EvidenceNote != original.EvidenceNote {
		t.Fatalf("rejected draft leaked into repository projection: got %+v, want %+v", got, original)
	}
}
