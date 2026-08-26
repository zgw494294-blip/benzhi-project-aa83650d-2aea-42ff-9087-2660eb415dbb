package domain

import (
	"errors"
	"testing"
	"time"
)

func testBatch(t *testing.T) *ReleaseBatch {
	t.Helper()
	batch, err := NewReleaseBatch("batch-1", "方言点", "IPA", []string{"i", "a"}, 100, true, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	return batch
}

func TestFreezeProtectsScopeAndVersion(t *testing.T) {
	batch := testBatch(t)
	if err := batch.AddSegment(RecordingSegment{ID: "seg-1", SourceRef: "src", StartMillis: 0, EndMillis: 1000, SpeakerCode: "S1", PromptText: "文本"}); err != nil {
		t.Fatal(err)
	}
	if batch.Version != 2 {
		t.Fatalf("登记后的版本 = %d", batch.Version)
	}
	if err := batch.Freeze(); err != nil {
		t.Fatal(err)
	}
	err := batch.AddSegment(RecordingSegment{ID: "seg-2", SourceRef: "src", StartMillis: 1000, EndMillis: 2000, SpeakerCode: "S1", PromptText: "文本"})
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("冻结后登记错误 = %v", err)
	}
}

func TestOverlapAndAnnotatorIsolation(t *testing.T) {
	batch := testBatch(t)
	first := RecordingSegment{ID: "seg-1", SourceRef: "same", StartMillis: 0, EndMillis: 500, SpeakerCode: "S1", PromptText: "甲"}
	if err := batch.AddSegment(first); err != nil {
		t.Fatal(err)
	}
	overlap := RecordingSegment{ID: "seg-2", SourceRef: "same", StartMillis: 400, EndMillis: 800, SpeakerCode: "S2", PromptText: "乙"}
	if err := batch.AddSegment(overlap); err == nil {
		t.Fatal("预期区间重叠被拒绝")
	}
	if err := batch.Freeze(); err != nil {
		t.Fatal(err)
	}
	if err := batch.Assign("seg-1", "ann", "ann"); err == nil {
		t.Fatal("预期相同标注员被拒绝")
	}
}

func TestSealedBatchRejectsWrites(t *testing.T) {
	batch := testBatch(t)
	instant := time.Unix(200, 0)
	batch.State = StateSealed
	batch.SealedAt = &instant
	batch.Credential = &ReleaseCredential{ID: "cred"}
	if !errors.Is(batch.EnsureWritable(), ErrSealed) {
		t.Fatal("封存批次仍可写")
	}
}
