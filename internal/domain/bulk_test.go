package domain

import (
	"errors"
	"testing"
	"time"
)

func TestSegmentPreflightCollectsProblemsWithoutMutation(t *testing.T) {
	batch := testBatch(t)
	if err := batch.AddSegment(RecordingSegment{ID: "existing", SourceRef: "R", StartMillis: 0, EndMillis: 500, SpeakerCode: "S", PromptText: "已有"}); err != nil {
		t.Fatal(err)
	}
	version, count := batch.Version, len(batch.Segments)
	result, err := batch.PreflightSegments([]RecordingSegment{
		{ID: "missing", SourceRef: "A", StartMillis: 0, EndMillis: 200, PromptText: "缺说话人"},
		{ID: "short", SourceRef: "B", StartMillis: 0, EndMillis: 50, SpeakerCode: "S", PromptText: "过短"},
		{ID: "overlap", SourceRef: "R", StartMillis: 400, EndMillis: 700, SpeakerCode: "S", PromptText: "重叠"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid || len(result.Problems) != 3 {
		t.Fatalf("预检问题 = %+v", result.Problems)
	}
	if batch.Version != version || len(batch.Segments) != count {
		t.Fatal("预检改变了批次")
	}
}

func TestBulkSegmentsAndAssignmentsIncrementOnce(t *testing.T) {
	batch := testBatch(t)
	segments := []RecordingSegment{{ID: "s1", SourceRef: "R", StartMillis: 0, EndMillis: 100, SpeakerCode: "A", PromptText: "一"}, {ID: "s2", SourceRef: "R", StartMillis: 100, EndMillis: 200, SpeakerCode: "A", PromptText: "二"}}
	if err := batch.AddSegments(segments); err != nil {
		t.Fatal(err)
	}
	if batch.Version != 2 || batch.Segments["s1"].Ordinal != 1 || batch.Segments["s2"].Ordinal != 2 {
		t.Fatalf("批量登记结果异常：%+v", batch)
	}
	if err := batch.Freeze(); err != nil {
		t.Fatal(err)
	}
	preview, err := batch.PreviewAssignments([]AssignmentPlanItem{{SegmentID: "s1", First: "a", Second: "b"}, {SegmentID: "s2", First: "a", Second: "c"}})
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Valid || len(preview.Unassigned) != 0 {
		t.Fatalf("分配预览异常：%+v", preview)
	}
	before := batch.Version
	if err := batch.AssignMany(preview.Plan); err != nil {
		t.Fatal(err)
	}
	if batch.Version != before+1 || len(batch.Assignments) != 2 || batch.State != StateAnnotating {
		t.Fatalf("批量分配异常：%+v", batch)
	}
}

func TestBulkDecisionIsAllOrNothing(t *testing.T) {
	batch := testBatch(t)
	batch.State = StateAdjudicating
	batch.Decisions = map[string]AdjudicationDecision{
		DecisionKey("s1", "0-100"): {SegmentID: "s1", IntervalKey: "0-100", CandidateLabels: []string{"a", "i"}},
		DecisionKey("s2", "0-100"): {SegmentID: "s2", IntervalKey: "0-100", CandidateLabels: []string{"a", "i"}},
	}
	version := batch.Version
	err := batch.DecideMany([]BulkDecision{{SegmentID: "s1", IntervalKey: "0-100", ResolvedLabel: "a", Reason: "证据", AdjudicatorID: "judge"}, {SegmentID: "s2", IntervalKey: "0-100", ResolvedLabel: "x", Reason: "证据", AdjudicatorID: "judge"}}, "judge", time.Unix(2, 0))
	var validation BatchValidationError
	if !errors.As(err, &validation) || len(validation.Problems) != 1 {
		t.Fatalf("预期行级错误，得到 %v", err)
	}
	if batch.Version != version || batch.Decisions[DecisionKey("s1", "0-100")].DecidedAt != nil {
		t.Fatal("非法批量裁定发生部分写入")
	}
}

func TestEffectiveFindingsRetainSegmentsOutsideTargetedRun(t *testing.T) {
	batch := testBatch(t)
	batch.VerificationRuns = []VerificationRun{
		{ID: "full", Findings: []Finding{{Rule: "boundary", SegmentID: "A", Severity: "error"}, {Rule: "boundary", SegmentID: "B", Severity: "error"}}},
		{ID: "target-A", Scope: []string{"A"}, PreviousRunID: "full", Passed: true},
	}
	findings := batch.EffectiveFindings()
	if len(findings) != 1 || findings[0].SegmentID != "B" || batch.EffectiveVerificationPassed() {
		t.Fatalf("有效检查结果异常：%+v", findings)
	}
	batch.State = StateCandidate
	if err := batch.Seal(ReleaseCredential{ID: "c"}, "reviewer", time.Unix(3, 0)); err == nil {
		t.Fatal("未重检片段仍有错误时允许封存")
	}
}
