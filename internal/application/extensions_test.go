package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"acousticverdictworkbench/internal/domain"
	"acousticverdictworkbench/internal/quality"
	"acousticverdictworkbench/internal/repository"
)

func TestBulkRegisterClipsIsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	now := time.Date(2026, 8, 26, 5, 0, 0, 0, time.UTC)
	sequence := 0
	service := NewConfigured(repo, func() time.Time { return now }, func(prefix string) string {
		sequence++
		return prefix + "-" + itoaTest(sequence)
	})
	created, err := service.CreateBatch(ctx, CreateBatchCommand{Metadata: Metadata{ActorID: "manager", Role: "manager", IdempotencyKey: "create"}, Title: "批量测试", SiteCode: "SITE", RecordingStart: now, RecordingEnd: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	configured, err := service.ConfigureScope(ctx, ConfigureScopeCommand{Metadata: Metadata{ActorID: "manager", Role: "manager", ExpectedVersion: created.Version, IdempotencyKey: "scope"}, BatchID: created.ID, Title: created.Title, SiteCode: created.SiteCode, RecordingStart: now, RecordingEnd: now.Add(time.Hour), AllowedSpeciesCodes: []string{"BIRD_A"}})
	if err != nil {
		t.Fatal(err)
	}
	command := BulkRegisterClipsCommand{Metadata: Metadata{ActorID: "manager", Role: "manager", ExpectedVersion: configured.Version, IdempotencyKey: "bulk"}, BatchID: created.ID, Clips: []BulkClipInput{{ID: "clip-a", SourceName: "a.wav", CapturedAt: now.Add(time.Minute), DurationMs: 1000, ContentSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Sequence: 1}, {ID: "clip-b", SourceName: "b.wav", CapturedAt: now.Add(2 * time.Minute), DurationMs: 1000, ContentSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Sequence: 2}, {ID: "clip-c", SourceName: "c.wav", CapturedAt: now.Add(3 * time.Minute), DurationMs: 1000, ContentSHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", Sequence: 3}}}
	first, err := service.BulkRegisterClips(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.BulkRegisterClips(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if first.AddedCount != 3 || second.AddedCount != 3 || first.Batch.Version != configured.Version+1 || second.Batch.Version != first.Batch.Version || len(second.Batch.Clips) != 3 {
		t.Fatalf("幂等批量登记结果错误：first=%+v second=%+v", first, second)
	}
}

func TestManifestQueriesAreReadOnlyAndFilterStableEvents(t *testing.T) {
	ctx := context.Background()
	repo, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	now := time.Date(2026, 8, 26, 6, 0, 0, 0, time.UTC)
	event := domain.CandidateEvent{ID: "event", SubmissionID: "submission", SpeciesCode: "BIRD_A", StartMs: 100, EndMs: 300, Confidence: domain.ConfidenceHigh, EvidenceNote: "清晰鸣声"}
	source := &domain.ReviewBatch{ID: "released", Version: 6, Clips: []domain.AudioClip{{ID: "clip", SourceName: "field.wav", DurationMs: 1000, ContentSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}, Submissions: []domain.AnnotationSubmission{{ID: "submission", ClipID: "clip", Status: domain.SubmissionSubmitted, Events: []domain.CandidateEvent{event}}}, Disputes: []domain.DisputeCase{{ID: "agreement", ClipID: "clip", Kind: domain.DisputeAgreement, LeftEventID: "event", Status: domain.DisputeResolved}}}
	manifest, err := quality.BuildManifest("manifest", "publisher", source, now)
	if err != nil {
		t.Fatal(err)
	}
	stored := &domain.ReviewBatch{ID: "released", Status: domain.BatchReleased, Version: 7, Manifest: &manifest}
	if _, err := repo.Commit(ctx, repository.CommitRequest{Batch: stored, ExpectedVersion: 0, Operation: "test.seed", CommittedAt: now}); err != nil {
		t.Fatal(err)
	}
	service := New(repo)
	meta := Metadata{ActorID: "publisher", Role: "release_manager"}
	details, err := service.ManifestDetails(ctx, stored.ID, meta, ManifestEventQuery{ClipID: "clip", SpeciesCode: "bird_a", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	verification, err := service.VerifyManifest(ctx, stored.ID, meta)
	if err != nil {
		t.Fatal(err)
	}
	after, err := repo.Get(ctx, stored.ID)
	if err != nil {
		t.Fatal(err)
	}
	if details.Total != 1 || len(details.Events) != 1 || !verification.Consistent || after.Version != stored.Version {
		t.Fatalf("清单查询、核验或只读语义错误：details=%+v verification=%+v version=%d", details, verification, after.Version)
	}
	_, err = service.ManifestDetails(ctx, stored.ID, meta, ManifestEventQuery{Page: 0, PageSize: 10})
	if err == nil {
		t.Fatal("非法分页未被拒绝")
	}
	draft := &domain.ReviewBatch{ID: "draft", Status: domain.BatchDraft, Version: 1}
	if _, err := repo.Commit(ctx, repository.CommitRequest{Batch: draft, ExpectedVersion: 0, Operation: "test.seed", CommittedAt: now}); err != nil {
		t.Fatal(err)
	}
	_, err = service.ManifestDetails(ctx, draft.ID, meta, ManifestEventQuery{Page: 1, PageSize: 10})
	if !errors.Is(err, domain.ErrStateConflict) {
		t.Fatalf("未发布批次查询未返回状态冲突：%v", err)
	}
}

func itoaTest(value int) string {
	if value == 0 {
		return "0"
	}
	buffer := [20]byte{}
	position := len(buffer)
	for value > 0 {
		position--
		buffer[position] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[position:])
}
