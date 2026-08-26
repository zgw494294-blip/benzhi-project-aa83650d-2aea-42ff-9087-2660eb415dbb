package quality

import (
	"testing"
	"time"

	"acousticverdictworkbench/internal/domain"
)

func TestCheckerLocatesCoverageAndOpenDisputes(t *testing.T) {
	batch := &domain.ReviewBatch{ID: "batch", Version: 5, AllowedSpeciesCodes: []string{"BIRD_A"}, Clips: []domain.AudioClip{{ID: "clip", DurationMs: 1000}}, Disputes: []domain.DisputeCase{{ID: "case", ClipID: "clip", Kind: domain.DisputeLeftOnly, Status: domain.DisputeOpen}}}
	report := NewChecker().Check(batch, time.Now())
	if report.Passed || len(report.Issues) != 2 {
		t.Fatalf("应返回覆盖和分歧阻断：%+v", report)
	}
	if report.Issues[0].ClipID != "clip" || report.Issues[1].ClipID != "clip" {
		t.Fatal("阻断项不可定位")
	}
}
