package qualityclipcachebleed_test

import (
	"context"
	"testing"
	"time"

	"acousticverdictworkbench/internal/application"
	"acousticverdictworkbench/internal/domain"
	"acousticverdictworkbench/internal/repository"
)

func TestQualityCacheMustIsolateBatchesSharingClipID(t *testing.T) {
	repo, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	service := application.NewConfigured(repo, func() time.Time { return now }, func(prefix string) string { return prefix + "-fixed" })
	complete := qualityBatch("batch-complete", "clip-shared", 1, 2)
	incomplete := qualityBatch("batch-incomplete", "clip-shared", 1, 1)
	seedBatch(t, repo, complete, now)
	seedBatch(t, repo, incomplete, now)

	first, err := service.CheckQuality(context.Background(), application.BatchCommand{
		Metadata: application.Metadata{ActorID: "reviewer-a", Role: "reviewer", ExpectedVersion: complete.Version, IdempotencyKey: "check-complete"},
		BatchID:  complete.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.LastQuality == nil || !first.LastQuality.Passed {
		t.Fatalf("测试前提无效：完整批次未通过质量检查：%+v", first.LastQuality)
	}

	second, err := service.CheckQuality(context.Background(), application.BatchCommand{
		Metadata: application.Metadata{ActorID: "reviewer-a", Role: "reviewer", ExpectedVersion: incomplete.Version, IdempotencyKey: "check-incomplete"},
		BatchID:  incomplete.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.LastQuality == nil || second.LastQuality.Passed || second.LastQuality.CoveredClips != 0 {
		t.Fatalf("相同 clip ID 复用了另一批次的质量输入：%+v", second.LastQuality)
	}
	persisted, err := repo.Get(context.Background(), incomplete.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.LastQuality == nil || persisted.LastQuality.Passed {
		t.Fatalf("仓储未保留预期的阻断报告：%+v", persisted.LastQuality)
	}
}

func qualityBatch(batchID, clipID string, version uint64, submissionCount int) *domain.ReviewBatch {
	submissions := make([]domain.AnnotationSubmission, submissionCount)
	for i := range submissions {
		submissions[i] = domain.AnnotationSubmission{
			ID:          "submission-" + string(rune('a'+i)),
			ClipID:      clipID,
			AnnotatorID: "annotator-" + string(rune('a'+i)),
			Round:       1,
			Status:      domain.SubmissionSubmitted,
			Events: []domain.CandidateEvent{{
				ID:           "event-" + string(rune('a'+i)),
				SubmissionID: "submission-" + string(rune('a'+i)),
				SpeciesCode:  "BIRD_A",
				StartMs:      100,
				EndMs:        300,
				Confidence:   domain.ConfidenceHigh,
				EvidenceNote: "清晰鸣声",
			}},
		}
	}
	return &domain.ReviewBatch{
		ID:                  batchID,
		Status:              domain.BatchReady,
		Version:             version,
		AllowedSpeciesCodes: []string{"BIRD_A"},
		Clips:               []domain.AudioClip{{ID: clipID, BatchID: batchID, DurationMs: 1000}},
		Submissions:         submissions,
	}
}

func seedBatch(t *testing.T, repo *repository.Store, batch *domain.ReviewBatch, now time.Time) {
	t.Helper()
	_, err := repo.Commit(context.Background(), repository.CommitRequest{Batch: batch, ExpectedVersion: 0, Operation: "test.seed", CommittedAt: now})
	if err != nil {
		t.Fatal(err)
	}
}
