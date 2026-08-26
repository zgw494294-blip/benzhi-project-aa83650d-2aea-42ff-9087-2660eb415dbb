package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"phonemereleasedesk/internal/domain"
	"phonemereleasedesk/internal/persistence"
)

func TestBulkSegmentConfirmationIsIdempotent(t *testing.T) {
	repo, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := NewConfigured(repo, func() time.Time { return time.Unix(10, 0) }, func(prefix string) string { return prefix + "-id" })
	ctx := context.Background()
	batch, err := service.CreateBatch(ctx, CreateBatchCommand{Metadata: Metadata{Role: "manager", ActorID: "m", IdempotencyKey: "create"}, ID: "b", DialectSite: "点", PhoneticSystem: "IPA", AllowedLabels: []string{"a"}, MinimumSegmentMillis: 100, RequireDual: true})
	if err != nil {
		t.Fatal(err)
	}
	segments := []domain.RecordingSegment{{ID: " s1 ", SourceRef: " R ", StartMillis: 0, EndMillis: 100, SpeakerCode: " S ", PromptText: " 文本 "}}
	preflight, err := service.PreflightSegments(ctx, SegmentBatchCommand{Metadata: Metadata{Role: "manager", ActorID: "m", ExpectedVersion: batch.Version}, BatchID: "b", Segments: segments})
	if err != nil {
		t.Fatal(err)
	}
	command := SegmentBatchCommand{Metadata: Metadata{Role: "manager", ActorID: "m", ExpectedVersion: batch.Version, IdempotencyKey: "bulk-1"}, BatchID: "b", Segments: segments, ContentDigest: preflight.ContentDigest}
	first, err := service.AddSegments(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := service.AddSegments(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != retry.Version || len(retry.Segments) != 1 || retry.Segments["s1"].Ordinal != 1 {
		t.Fatalf("幂等结果异常：%+v", retry)
	}
}

func TestBulkSegmentConfirmationRejectsStalePreflight(t *testing.T) {
	repo, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := NewConfigured(repo, func() time.Time { return time.Unix(10, 0) }, func(prefix string) string { return prefix + "-id" })
	ctx := context.Background()
	batch, err := service.CreateBatch(ctx, CreateBatchCommand{Metadata: Metadata{Role: "manager", ActorID: "m", IdempotencyKey: "create"}, ID: "b", DialectSite: "点", PhoneticSystem: "IPA", AllowedLabels: []string{"a"}, MinimumSegmentMillis: 100, RequireDual: true})
	if err != nil {
		t.Fatal(err)
	}
	segments := []domain.RecordingSegment{{ID: "bulk", SourceRef: "R", StartMillis: 100, EndMillis: 200, SpeakerCode: "S", PromptText: "批量"}}
	preflight, err := service.PreflightSegments(ctx, SegmentBatchCommand{Metadata: Metadata{Role: "manager", ActorID: "m", ExpectedVersion: batch.Version}, BatchID: "b", Segments: segments})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.AddSegment(ctx, SegmentCommand{Metadata: Metadata{Role: "manager", ActorID: "m", ExpectedVersion: batch.Version, IdempotencyKey: "single"}, BatchID: "b", ID: "other", SourceRef: "R", StartMillis: 0, EndMillis: 100, SpeakerCode: "S", PromptText: "其他"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.AddSegments(ctx, SegmentBatchCommand{Metadata: Metadata{Role: "manager", ActorID: "m", ExpectedVersion: batch.Version, IdempotencyKey: "bulk"}, BatchID: "b", Segments: segments, ContentDigest: preflight.ContentDigest})
	if !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("预期版本冲突，得到 %v", err)
	}
	loaded, err := service.GetBatch(ctx, "b")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != updated.Version || len(loaded.Segments) != 1 {
		t.Fatal("过期确认发生了部分登记")
	}
}
