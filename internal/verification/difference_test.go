package verification

import (
	"testing"

	"phonemereleasedesk/internal/domain"
)

func TestCompareRunsPreservesFindingsOutsideTargetScope(t *testing.T) {
	before := domain.VerificationRun{ID: "full", Findings: []domain.Finding{{Rule: "boundary", SegmentID: "A", Severity: "error"}, {Rule: "boundary", SegmentID: "B", Severity: "error"}}}
	after := domain.VerificationRun{ID: "target", Scope: []string{"A"}, PreviousRunID: "full"}
	result := CompareRuns(before, after)
	if result.Resolved != 1 || result.Persisting != 1 || result.Added != 0 {
		t.Fatalf("范围感知差异异常：%+v", result)
	}
	for _, change := range result.Changes {
		if change.Finding.SegmentID == "B" && change.Note == "" {
			t.Fatal("未重检片段缺少保留状态说明")
		}
	}
}
