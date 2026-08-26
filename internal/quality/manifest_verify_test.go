package quality

import (
	"testing"
	"time"

	"acousticverdictworkbench/internal/domain"
)

func TestVerifyManifestLocatesDigestMismatch(t *testing.T) {
	now := time.Date(2026, 8, 26, 4, 0, 0, 0, time.UTC)
	batch := &domain.ReviewBatch{ID: "batch", Version: 9, Clips: []domain.AudioClip{{ID: "clip", SourceName: "field.wav", DurationMs: 1000, ContentSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}, AdjudicationTrail: []domain.AdjudicationRecord{{ID: "trail", DisputeID: "dispute", ClipID: "clip", Kind: domain.ResolutionNoEvent, ReviewerID: "reviewer", Reason: "无有效事件", At: now}}}
	manifest, err := BuildManifest("manifest", "publisher", batch, now)
	if err != nil {
		t.Fatal(err)
	}
	checks, err := VerifyManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range checks {
		if !check.Consistent {
			t.Fatalf("新清单核验失败：%+v", checks)
		}
	}
	manifest.SourceClips[0].SourceName = "tampered.wav"
	checks, err = VerifyManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if checks[0].Consistent || checks[2].Consistent || !checks[1].Consistent {
		t.Fatalf("未准确定位片段与清单摘要不一致：%+v", checks)
	}
}
