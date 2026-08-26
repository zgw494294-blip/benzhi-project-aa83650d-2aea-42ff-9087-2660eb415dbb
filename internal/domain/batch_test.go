package domain

import (
	"errors"
	"testing"
	"time"
)

func TestFreezeProtectsScopeAndIsolatesDrafts(t *testing.T) {
	now := time.Date(2026, 8, 26, 1, 0, 0, 0, time.UTC)
	batch, err := NewReviewBatch("batch-1", "林区晨间录音", "SITE-A", now, now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := batch.ConfigureScope(batch.Title, batch.SiteCode, now, now.Add(time.Hour), []string{"bird_a"}, now); err != nil {
		t.Fatal(err)
	}
	clip := AudioClip{ID: "clip-1", SourceName: "a.wav", CapturedAt: now.Add(time.Minute), DurationMs: 3000, ContentSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Sequence: 1}
	if err := batch.AddClip(clip, now); err != nil {
		t.Fatal(err)
	}
	if err := batch.Freeze(now); err != nil {
		t.Fatal(err)
	}
	if err := batch.AddClip(clip, now); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("冻结后仍可添加片段：%v", err)
	}
	events := []CandidateEvent{{ID: "event-1", SpeciesCode: "BIRD_A", StartMs: 100, EndMs: 500, Confidence: ConfidenceHigh, EvidenceNote: "两节清晰鸣声"}}
	if err := batch.SaveDraft("sub-1", "clip-1", "ann-a", 1, events, "", now); err != nil {
		t.Fatal(err)
	}
	visible := batch.VisibleSubmissions("ann-b", "annotator")
	if len(visible) != 0 {
		t.Fatalf("未提交草稿泄漏给另一标注员：%d", len(visible))
	}
	if err := batch.Submit("clip-1", "ann-a", 1, now); err != nil {
		t.Fatal(err)
	}
	if len(batch.VisibleSubmissions("ann-b", "annotator")) != 1 {
		t.Fatal("提交后仍不可见")
	}
}

func TestResolutionReturnOnlyReopensTarget(t *testing.T) {
	now := time.Now().UTC()
	b := &ReviewBatch{ID: "b", Status: BatchAdjudicating, Version: 4, AllowedSpeciesCodes: []string{"BIRD_A"}, Clips: []AudioClip{{ID: "c", DurationMs: 1000}}, Submissions: []AnnotationSubmission{{ID: "sa", ClipID: "c", AnnotatorID: "a", Round: 1, Status: SubmissionSubmitted}, {ID: "sb", ClipID: "c", AnnotatorID: "b", Round: 1, Status: SubmissionSubmitted}}, Disputes: []DisputeCase{{ID: "d", ClipID: "c", Kind: DisputeConflict, Status: DisputeOpen}}}
	err := b.ResolveDispute("d", "reviewer", Resolution{Kind: ResolutionReturn, ReturnAnnotator: "a", Reason: "需要复听"}, "trail", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Submissions) != 3 || b.Submissions[2].AnnotatorID != "a" || b.Submissions[2].Round != 2 || b.Submissions[2].Status != SubmissionReopened {
		t.Fatalf("返标范围错误：%+v", b.Submissions)
	}
	if b.Submissions[1].Status != SubmissionSubmitted {
		t.Fatal("未命中的标注员被错误重开")
	}
}
