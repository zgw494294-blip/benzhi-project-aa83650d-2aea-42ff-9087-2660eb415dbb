package matchcache_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"acousticverdictworkbench/internal/application"
	"acousticverdictworkbench/internal/domain"
	"acousticverdictworkbench/internal/repository"
)

func TestConcurrentMatchesMustIsolateMemoization(t *testing.T) {
	ctx := context.Background()
	repo, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	for _, fixture := range []struct {
		batchID string
		clipID  string
	}{
		{batchID: "batch-north", clipID: "clip-north"},
		{batchID: "batch-south", clipID: "clip-south"},
	} {
		batch := matchReadyBatch(fixture.batchID, fixture.clipID, now)
		if _, err := repo.Commit(ctx, repository.CommitRequest{Batch: batch, ExpectedVersion: 0, Operation: "test.seed", CommittedAt: now}); err != nil {
			t.Fatal(err)
		}
	}

	arrived := make(chan struct{}, 2)
	release := make(chan struct{})
	var idMu sync.Mutex
	nextID := 0
	service := application.NewConfigured(repo, func() time.Time { return now }, func(prefix string) string {
		arrived <- struct{}{}
		<-release
		idMu.Lock()
		defer idMu.Unlock()
		nextID++
		return fmt.Sprintf("%s-%d", prefix, nextID)
	})

	start := make(chan struct{})
	errors := make(chan error, 2)
	var workers sync.WaitGroup
	for _, fixture := range []struct {
		batchID string
		clipID  string
	}{
		{batchID: "batch-north", clipID: "clip-north"},
		{batchID: "batch-south", clipID: "clip-south"},
	} {
		fixture := fixture
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, submitErr := service.Submit(ctx, application.SubmitCommand{
				Metadata:    application.Metadata{ActorID: "annotator-b", Role: "annotator", ExpectedVersion: 1, IdempotencyKey: "submit-" + fixture.batchID},
				BatchID:     fixture.batchID,
				ClipID:      fixture.clipID,
				AnnotatorID: "annotator-b",
				Round:       1,
				Confirmed:   true,
			})
			errors <- submitErr
		}()
	}

	close(start)
	<-arrived
	<-arrived
	close(release)
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("并发提交失败：%v", err)
		}
	}
}

func matchReadyBatch(batchID, clipID string, now time.Time) *domain.ReviewBatch {
	leftSubmissionID := "submission-left-" + clipID
	rightSubmissionID := "submission-right-" + clipID
	submittedAt := now.Add(-time.Minute)
	return &domain.ReviewBatch{
		ID:                  batchID,
		Title:               "并发匹配复现",
		SiteCode:            "SITE-RACE",
		RecordingStart:      now.Add(-time.Hour),
		RecordingEnd:        now.Add(time.Hour),
		AllowedSpeciesCodes: []string{"BIRD_A"},
		Status:              domain.BatchAnnotating,
		Version:             1,
		CreatedAt:           now.Add(-2 * time.Hour),
		Clips: []domain.AudioClip{{
			ID: clipID, BatchID: batchID, SourceName: clipID + ".wav", CapturedAt: now,
			DurationMs: 1000, ContentSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Sequence: 1,
		}},
		Submissions: []domain.AnnotationSubmission{
			{
				ID: leftSubmissionID, ClipID: clipID, AnnotatorID: "annotator-a", Round: 1,
				Status: domain.SubmissionSubmitted, SubmittedAt: &submittedAt,
				Events: []domain.CandidateEvent{{ID: "event-left-" + clipID, SubmissionID: leftSubmissionID, SpeciesCode: "BIRD_A", StartMs: 100, EndMs: 500, Confidence: domain.ConfidenceHigh, EvidenceNote: "左声道清晰鸣声"}},
			},
			{
				ID: rightSubmissionID, ClipID: clipID, AnnotatorID: "annotator-b", Round: 1,
				Status: domain.SubmissionDraft,
				Events: []domain.CandidateEvent{{ID: "event-right-" + clipID, SubmissionID: rightSubmissionID, SpeciesCode: "BIRD_A", StartMs: 120, EndMs: 480, Confidence: domain.ConfidenceHigh, EvidenceNote: "右声道清晰鸣声"}},
			},
		},
	}
}
