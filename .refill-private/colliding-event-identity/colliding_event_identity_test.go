package collidingeventidentity_test

import (
	"testing"
	"time"

	"acousticverdictworkbench/internal/domain"
	"acousticverdictworkbench/internal/quality"
)

func TestAcceptRightUsesRightSubmissionWhenEventIDsCollide(t *testing.T) {
	now := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	batch, err := domain.NewReviewBatch("batch", "碰撞测试", "SITE", now, now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := batch.ConfigureScope("碰撞测试", "SITE", now, now.Add(time.Hour), []string{"BIRD_A", "BIRD_B"}, now); err != nil {
		t.Fatal(err)
	}
	if err := batch.AddClip(domain.AudioClip{ID: "clip", SourceName: "clip.wav", CapturedAt: now, DurationMs: 5000, ContentSHA256: strings64("a"), Sequence: 1}, now); err != nil {
		t.Fatal(err)
	}
	if err := batch.Freeze(now); err != nil {
		t.Fatal(err)
	}
	left := domain.CandidateEvent{ID: "shared-event", SpeciesCode: "BIRD_A", StartMs: 100, EndMs: 1000, Confidence: domain.ConfidenceHigh, EvidenceNote: "left"}
	right := domain.CandidateEvent{ID: "shared-event", SpeciesCode: "BIRD_B", StartMs: 100, EndMs: 1000, Confidence: domain.ConfidenceHigh, EvidenceNote: "right"}
	if err := batch.SaveDraft("left-sub", "clip", "alice", 1, []domain.CandidateEvent{left}, "", now); err != nil {
		t.Fatal(err)
	}
	if err := batch.Submit("clip", "alice", 1, now); err != nil {
		t.Fatal(err)
	}
	if err := batch.SaveDraft("right-sub", "clip", "bob", 1, []domain.CandidateEvent{right}, "", now); err != nil {
		t.Fatal(err)
	}
	if err := batch.Submit("clip", "bob", 1, now); err != nil {
		t.Fatal(err)
	}
	seq := 0
	cases := quality.NewMatcher().Match("clip", batch.LatestSubmitted("clip")[0], batch.LatestSubmitted("clip")[1], func(string) string {
		seq++
		return "case"
	})
	if len(cases) != 1 || cases[0].Kind != domain.DisputeConflict {
		t.Fatalf("未形成预期分歧：%+v", cases)
	}
	if err := batch.ReplaceClipDisputes("clip", cases, now); err != nil {
		t.Fatal(err)
	}
	if err := batch.ResolveDispute("case", "reviewer", domain.Resolution{Kind: domain.ResolutionAcceptRight, Reason: "采用右方"}, "record", now); err != nil {
		t.Fatal(err)
	}
	resolved, err := batch.Dispute("case")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Resolution == nil || resolved.Resolution.NormalizedEvent == nil || resolved.Resolution.NormalizedEvent.SpeciesCode != "BIRD_B" {
		t.Fatalf("TestAcceptRightUsesRightSubmissionWhenEventIDsCollide: accept_right 未采用右方 BIRD_B 事件：%+v", resolved.Resolution)
	}
}

func strings64(value string) string {
	result := ""
	for i := 0; i < 64; i++ {
		result += value
	}
	return result
}
