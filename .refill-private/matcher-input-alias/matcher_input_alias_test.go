package matcher_input_alias_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"acousticverdictworkbench/internal/application"
	"acousticverdictworkbench/internal/domain"
	"acousticverdictworkbench/internal/repository"
)

func TestMatchingMustNotReorderPersistedSubmission(t *testing.T) {
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	now := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	nextID := 0
	service := application.NewConfigured(store, func() time.Time { return now }, func(prefix string) string {
		nextID++
		return fmt.Sprintf("%s-%d", prefix, nextID)
	})
	ctx := context.Background()
	manager := application.Metadata{ActorID: "manager-a", Role: "manager"}

	batch, err := service.CreateBatch(ctx, application.CreateBatchCommand{
		Metadata:       withKey(manager, "create"),
		Title:          "输入所有权复现批次",
		SiteCode:       "SITE-A",
		RecordingStart: now,
		RecordingEnd:   now.Add(time.Hour),
	})
	must(t, err)
	batch, err = service.ConfigureScope(ctx, application.ConfigureScopeCommand{
		Metadata:            withVersionAndKey(manager, batch.Version, "scope"),
		BatchID:             batch.ID,
		Title:               batch.Title,
		SiteCode:            batch.SiteCode,
		RecordingStart:      batch.RecordingStart,
		RecordingEnd:        batch.RecordingEnd,
		AllowedSpeciesCodes: []string{"BIRD_A"},
	})
	must(t, err)
	batch, err = service.AddClip(ctx, application.AddClipCommand{
		Metadata:      withVersionAndKey(manager, batch.Version, "clip"),
		BatchID:       batch.ID,
		ID:            "clip-a",
		SourceName:    "clip-a.wav",
		CapturedAt:    now.Add(time.Minute),
		DurationMs:    1000,
		ContentSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Sequence:      1,
	})
	must(t, err)
	batch, err = service.Freeze(ctx, application.BatchCommand{
		Metadata: withVersionAndKey(manager, batch.Version, "freeze"),
		BatchID:  batch.ID,
	})
	must(t, err)

	eventsA := []domain.CandidateEvent{
		{ID: "z-early", SpeciesCode: "BIRD_A", StartMs: 100, EndMs: 200, Confidence: domain.ConfidenceHigh, EvidenceNote: "较早事件"},
		{ID: "a-late", SpeciesCode: "BIRD_A", StartMs: 600, EndMs: 700, Confidence: domain.ConfidenceHigh, EvidenceNote: "较晚事件"},
	}
	eventsB := []domain.CandidateEvent{
		{ID: "y-early", SpeciesCode: "BIRD_A", StartMs: 100, EndMs: 200, Confidence: domain.ConfidenceHigh, EvidenceNote: "对应较早事件"},
		{ID: "b-late", SpeciesCode: "BIRD_A", StartMs: 600, EndMs: 700, Confidence: domain.ConfidenceHigh, EvidenceNote: "对应较晚事件"},
	}
	batch = saveDraft(t, ctx, service, batch, "annotator-a", "submission-a", "draft-a", eventsA)
	batch = saveDraft(t, ctx, service, batch, "annotator-b", "submission-b", "draft-b", eventsB)
	batch = submit(t, ctx, service, batch, "annotator-a", "submit-a")

	before, err := store.Get(ctx, batch.ID)
	must(t, err)
	assertSubmissionOrder(t, before, "annotator-a", []string{"z-early", "a-late"})

	batch = submit(t, ctx, service, batch, "annotator-b", "submit-b")
	after, err := store.Get(ctx, batch.ID)
	must(t, err)
	assertSubmissionOrder(t, after, "annotator-a", []string{"z-early", "a-late"})
}

func saveDraft(t *testing.T, ctx context.Context, service *application.Service, batch *domain.ReviewBatch, annotator, submissionID, key string, events []domain.CandidateEvent) *domain.ReviewBatch {
	t.Helper()
	result, err := service.SaveDraft(ctx, application.DraftCommand{
		Metadata:     withVersionAndKey(application.Metadata{ActorID: annotator, Role: "annotator"}, batch.Version, key),
		BatchID:      batch.ID,
		ClipID:       "clip-a",
		SubmissionID: submissionID,
		AnnotatorID:  annotator,
		Round:        1,
		Events:       events,
	})
	must(t, err)
	return result
}

func submit(t *testing.T, ctx context.Context, service *application.Service, batch *domain.ReviewBatch, annotator, key string) *domain.ReviewBatch {
	t.Helper()
	result, err := service.Submit(ctx, application.SubmitCommand{
		Metadata:    withVersionAndKey(application.Metadata{ActorID: annotator, Role: "annotator"}, batch.Version, key),
		BatchID:     batch.ID,
		ClipID:      "clip-a",
		AnnotatorID: annotator,
		Round:       1,
		Confirmed:   true,
	})
	must(t, err)
	return result
}

func assertSubmissionOrder(t *testing.T, batch *domain.ReviewBatch, annotator string, want []string) {
	t.Helper()
	for _, submission := range batch.Submissions {
		if submission.AnnotatorID != annotator {
			continue
		}
		if len(submission.Events) != len(want) {
			t.Fatalf("%s 的事件数为 %d，期望 %d", annotator, len(submission.Events), len(want))
		}
		for i, id := range want {
			if submission.Events[i].ID != id {
				t.Fatalf("%s 的第 %d 个事件变成 %s，期望保持 %s", annotator, i, submission.Events[i].ID, id)
			}
		}
		return
	}
	t.Fatalf("未找到 %s 的提交", annotator)
}

func withKey(meta application.Metadata, key string) application.Metadata {
	meta.IdempotencyKey = key
	return meta
}

func withVersionAndKey(meta application.Metadata, version uint64, key string) application.Metadata {
	meta.ExpectedVersion = version
	meta.IdempotencyKey = key
	return meta
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
