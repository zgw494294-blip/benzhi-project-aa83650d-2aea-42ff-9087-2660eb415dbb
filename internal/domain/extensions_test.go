package domain

import (
	"errors"
	"testing"
	"time"
)

func draftBatchForExtensions(t *testing.T) (*ReviewBatch, time.Time) {
	t.Helper()
	now := time.Date(2026, 8, 26, 2, 0, 0, 0, time.UTC)
	batch, err := NewReviewBatch("batch-ext", "扩展测试", "SITE-X", now, now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := batch.ConfigureScope(batch.Title, batch.SiteCode, now, now.Add(time.Hour), []string{"BIRD_A", "BIRD_B"}, now); err != nil {
		t.Fatal(err)
	}
	return batch, now
}

func TestAddClipsIsAtomicAndUsesOneBusinessVersion(t *testing.T) {
	batch, now := draftBatchForExtensions(t)
	before := batch.Version
	clips := []AudioClip{
		{ID: "clip-3", SourceName: "  three.wav  ", CapturedAt: now.Add(3 * time.Minute), DurationMs: 3000, ContentSHA256: "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC", Sequence: 3},
		{ID: "clip-1", SourceName: "one.wav", CapturedAt: now.Add(time.Minute), DurationMs: 1000, ContentSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Sequence: 1},
		{ID: "clip-2", SourceName: "two.wav", CapturedAt: now.Add(2 * time.Minute), DurationMs: 2000, ContentSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Sequence: 2},
	}
	if err := batch.AddClips(clips, now); err != nil {
		t.Fatal(err)
	}
	if batch.Version != before+1 || len(batch.Clips) != 3 {
		t.Fatalf("批量登记未以单版本提交：version=%d clips=%d", batch.Version, len(batch.Clips))
	}
	if batch.Clips[0].Sequence != 1 || batch.Clips[2].SourceName != "three.wav" || batch.Clips[2].ContentSHA256[0] != 'c' {
		t.Fatalf("批量规范化或排序错误：%+v", batch.Clips)
	}

	unchangedVersion, unchangedCount := batch.Version, len(batch.Clips)
	invalid := []AudioClip{
		{ID: "clip-4", SourceName: "four.wav", CapturedAt: now.Add(4 * time.Minute), DurationMs: 1000, ContentSHA256: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", Sequence: 4},
		{ID: "clip-5", SourceName: "five.wav", CapturedAt: now.Add(2 * time.Hour), DurationMs: 1000, ContentSHA256: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", Sequence: 4},
	}
	err := batch.AddClips(invalid, now)
	var validation *ValidationErrors
	if !errors.As(err, &validation) || len(validation.Errors) < 3 {
		t.Fatalf("未返回全部逐行冲突：%#v", err)
	}
	if batch.Version != unchangedVersion || len(batch.Clips) != unchangedCount {
		t.Fatal("批量预检失败后仍修改了聚合")
	}
}

func TestMultiEventDraftReturnsLocatedErrorsAndSorts(t *testing.T) {
	batch, now := draftBatchForExtensions(t)
	clip := AudioClip{ID: "clip", SourceName: "field.wav", CapturedAt: now.Add(time.Minute), DurationMs: 3000, ContentSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Sequence: 1}
	if err := batch.AddClip(clip, now); err != nil {
		t.Fatal(err)
	}
	if err := batch.Freeze(now); err != nil {
		t.Fatal(err)
	}
	before := batch.Version
	invalid := []CandidateEvent{
		{ID: "e1", SpeciesCode: "BIRD_A", StartMs: 10, EndMs: 4000, Confidence: ConfidenceHigh, EvidenceNote: "越界"},
		{ID: "e2", SpeciesCode: "OUTSIDE", StartMs: 10, EndMs: 20, Confidence: ConfidenceHigh, EvidenceNote: "范围外"},
		{ID: "e3", SpeciesCode: "BIRD_A", StartMs: 100, EndMs: 200, Confidence: ConfidenceHigh, EvidenceNote: "重复事件"},
		{ID: "e4", SpeciesCode: " bird_a ", StartMs: 100, EndMs: 200, Confidence: ConfidenceHigh, EvidenceNote: " 重复事件 "},
	}
	err := batch.SaveDraft("submission", "clip", "ann-a", 1, invalid, "", now)
	var validation *ValidationErrors
	if !errors.As(err, &validation) || len(validation.Errors) != 3 {
		t.Fatalf("多事件错误定位不完整：%#v", err)
	}
	if batch.Version != before || len(batch.Submissions) != 0 {
		t.Fatal("非法草稿改变了服务端版本")
	}
	valid := []CandidateEvent{
		{ID: "late", SpeciesCode: "BIRD_B", StartMs: 900, EndMs: 1100, Confidence: ConfidenceMedium, EvidenceNote: "后段事件"},
		{ID: "early", SpeciesCode: "BIRD_A", StartMs: 100, EndMs: 300, Confidence: ConfidenceHigh, EvidenceNote: "前段事件"},
		{ID: "middle", SpeciesCode: "BIRD_A", StartMs: 500, EndMs: 600, Confidence: ConfidenceLow, EvidenceNote: "中段事件"},
	}
	if err := batch.SaveDraft("submission", "clip", "ann-a", 1, valid, "", now); err != nil {
		t.Fatal(err)
	}
	if got := batch.Submissions[0].Events; len(got) != 3 || got[0].ID != "early" || got[2].ID != "late" {
		t.Fatalf("多事件草稿未稳定排序：%+v", got)
	}
}

func TestReannotationTaskLocksOwnershipRoundAndHistory(t *testing.T) {
	now := time.Date(2026, 8, 26, 3, 0, 0, 0, time.UTC)
	left := CandidateEvent{ID: "left", SubmissionID: "sa", SpeciesCode: "BIRD_A", StartMs: 100, EndMs: 500, Confidence: ConfidenceHigh, EvidenceNote: "左方"}
	right := CandidateEvent{ID: "right", SubmissionID: "sb", SpeciesCode: "BIRD_B", StartMs: 100, EndMs: 500, Confidence: ConfidenceHigh, EvidenceNote: "右方"}
	batch := &ReviewBatch{ID: "batch", Status: BatchAdjudicating, Version: 8, AllowedSpeciesCodes: []string{"BIRD_A", "BIRD_B"}, Clips: []AudioClip{{ID: "clip", DurationMs: 1000}}, Submissions: []AnnotationSubmission{{ID: "sa", ClipID: "clip", AnnotatorID: "ann-a", Round: 1, Status: SubmissionSubmitted, Events: []CandidateEvent{left}}, {ID: "sb", ClipID: "clip", AnnotatorID: "ann-b", Round: 1, Status: SubmissionSubmitted, Events: []CandidateEvent{right}}}, Disputes: []DisputeCase{{ID: "dispute", ClipID: "clip", Kind: DisputeConflict, LeftEventID: "left", RightEventID: "right", Status: DisputeOpen, Basis: MatchBasis{Explanation: "物种不同"}}}}
	resolution := Resolution{Kind: ResolutionReturn, ReturnAnnotator: "ann-b", Reason: "请复听右方事件"}
	if err := batch.ResolveDisputeWithTask("dispute", "reviewer", resolution, "trail", "task", "sb-r2", []string{"right-r2"}, now); err != nil {
		t.Fatal(err)
	}
	if len(batch.ReannotationTasks) != 1 || batch.ReannotationTasks[0].Round != 2 || batch.Submissions[1].Status != SubmissionSubmitted {
		t.Fatalf("返标任务或历史轮次不正确：%+v", batch)
	}
	if err := batch.SaveDraft("wrong", "clip", "ann-a", 2, []CandidateEvent{left}, "", now); !errors.Is(err, ErrTaskOwner) {
		t.Fatalf("错误标注员未被拒绝：%v", err)
	}
	revised := []CandidateEvent{{ID: "right-r2", SpeciesCode: "BIRD_A", StartMs: 120, EndMs: 520, Confidence: ConfidenceHigh, EvidenceNote: "复听后修订"}}
	if err := batch.SaveDraft("sb-r2", "clip", "ann-b", 2, revised, "", now); err != nil {
		t.Fatal(err)
	}
	if err := batch.SubmitConfirmed("clip", "ann-b", 2, "", true, now); err == nil {
		t.Fatal("返标提交未要求修订说明")
	}
	if err := batch.SubmitConfirmed("clip", "ann-b", 2, "修正物种与时间边界", true, now); err != nil {
		t.Fatal(err)
	}
	if batch.ReannotationTasks[0].Status != ReannotationClosed || batch.Submissions[1].Status != SubmissionSubmitted || batch.Submissions[2].Status != SubmissionSubmitted {
		t.Fatalf("返标闭环或历史保存错误：%+v", batch)
	}
	if err := batch.SubmitConfirmed("clip", "ann-b", 2, "再次提交", true, now); !errors.Is(err, ErrTaskClosed) {
		t.Fatalf("已关闭任务错误不可区分：%v", err)
	}
}
