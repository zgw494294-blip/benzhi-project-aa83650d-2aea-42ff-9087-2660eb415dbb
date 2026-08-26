package application

import (
	"context"

	"acousticverdictworkbench/internal/domain"
)

func (s *Service) CreateBatch(ctx context.Context, cmd CreateBatchCommand) (*domain.ReviewBatch, error) {
	if err := authorize(cmd.Metadata, "manager"); err != nil {
		return nil, err
	}
	if existing, ok := s.existing(ctx, "batch.create", cmd.Metadata); ok {
		return existing, nil
	}
	now := s.now()
	batch, err := domain.NewReviewBatch(s.ids("batch"), cmd.Title, cmd.SiteCode, cmd.RecordingStart, cmd.RecordingEnd, now)
	if err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, 0, "batch.create", cmd.IdempotencyKey)
}

func (s *Service) ConfigureScope(ctx context.Context, cmd ConfigureScopeCommand) (*domain.ReviewBatch, error) {
	if err := authorize(cmd.Metadata, "manager"); err != nil {
		return nil, err
	}
	if existing, ok := s.existing(ctx, "batch.configure", cmd.Metadata); ok {
		return existing, nil
	}
	batch, err := s.repo.Get(ctx, cmd.BatchID)
	if err != nil {
		return nil, err
	}
	if err := batch.ConfigureScope(cmd.Title, cmd.SiteCode, cmd.RecordingStart, cmd.RecordingEnd, cmd.AllowedSpeciesCodes, s.now()); err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, cmd.ExpectedVersion, "batch.configure", cmd.IdempotencyKey)
}

func (s *Service) AddClip(ctx context.Context, cmd AddClipCommand) (*domain.ReviewBatch, error) {
	if err := authorize(cmd.Metadata, "manager"); err != nil {
		return nil, err
	}
	if existing, ok := s.existing(ctx, "clip.add", cmd.Metadata); ok {
		return existing, nil
	}
	batch, err := s.repo.Get(ctx, cmd.BatchID)
	if err != nil {
		return nil, err
	}
	clip := domain.AudioClip{ID: cmd.ID, SourceName: cmd.SourceName, CapturedAt: cmd.CapturedAt, DurationMs: cmd.DurationMs, ContentSHA256: cmd.ContentSHA256, Sequence: cmd.Sequence}
	if clip.ID == "" {
		clip.ID = s.ids("clip")
	}
	if err := batch.AddClip(clip, s.now()); err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, cmd.ExpectedVersion, "clip.add", cmd.IdempotencyKey)
}

func (s *Service) BulkRegisterClips(ctx context.Context, cmd BulkRegisterClipsCommand) (*BulkRegisterClipsResult, error) {
	if err := authorize(cmd.Metadata, "manager"); err != nil {
		return nil, err
	}
	if len(cmd.Clips) == 0 || len(cmd.Clips) > 200 {
		return nil, domain.Invalid("clips", "批量登记条数必须在 1 到 200 之间")
	}
	if existing, ok := s.existing(ctx, "clip.bulk_register", cmd.Metadata); ok {
		return &BulkRegisterClipsResult{Batch: existing, AddedCount: len(cmd.Clips)}, nil
	}
	batch, err := s.repo.Get(ctx, cmd.BatchID)
	if err != nil {
		return nil, err
	}
	clips := make([]domain.AudioClip, len(cmd.Clips))
	for i, input := range cmd.Clips {
		id := input.ID
		if id == "" {
			id = s.ids("clip")
		}
		clips[i] = domain.AudioClip{ID: id, SourceName: input.SourceName, CapturedAt: input.CapturedAt, DurationMs: input.DurationMs, ContentSHA256: input.ContentSHA256, Sequence: input.Sequence}
	}
	if err := batch.AddClips(clips, s.now()); err != nil {
		return nil, err
	}
	committed, err := s.commit(ctx, batch, cmd.ExpectedVersion, "clip.bulk_register", cmd.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	return &BulkRegisterClipsResult{Batch: committed, AddedCount: len(clips)}, nil
}

func (s *Service) RemoveClip(ctx context.Context, cmd BatchCommand, clipID string) (*domain.ReviewBatch, error) {
	if err := authorize(cmd.Metadata, "manager"); err != nil {
		return nil, err
	}
	if existing, ok := s.existing(ctx, "clip.remove", cmd.Metadata); ok {
		return existing, nil
	}
	batch, err := s.repo.Get(ctx, cmd.BatchID)
	if err != nil {
		return nil, err
	}
	if err := batch.RemoveClip(clipID, s.now()); err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, cmd.ExpectedVersion, "clip.remove", cmd.IdempotencyKey)
}

func (s *Service) Freeze(ctx context.Context, cmd BatchCommand) (*domain.ReviewBatch, error) {
	if err := authorize(cmd.Metadata, "manager"); err != nil {
		return nil, err
	}
	if existing, ok := s.existing(ctx, "batch.freeze", cmd.Metadata); ok {
		return existing, nil
	}
	batch, err := s.repo.Get(ctx, cmd.BatchID)
	if err != nil {
		return nil, err
	}
	if err := batch.Freeze(s.now()); err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, cmd.ExpectedVersion, "batch.freeze", cmd.IdempotencyKey)
}
