package application

import (
	"context"

	"phonemereleasedesk/internal/domain"
)

func (s *Service) CreateBatch(ctx context.Context, command CreateBatchCommand) (*domain.ReleaseBatch, error) {
	if err := authorize(command.Metadata, "manager"); err != nil {
		return nil, err
	}
	if cached, ok := s.idempotent(ctx, "create-batch", command.IdempotencyKey); ok {
		return cached, nil
	}
	if command.ID == "" {
		command.ID = s.ids("batch")
	}
	batch, err := domain.NewReleaseBatch(command.ID, command.DialectSite, command.PhoneticSystem, command.AllowedLabels, command.MinimumSegmentMillis, command.RequireDual, s.now())
	if err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, 0, "create-batch", command.IdempotencyKey)
}

func (s *Service) AddSegment(ctx context.Context, command SegmentCommand) (*domain.ReleaseBatch, error) {
	if err := authorize(command.Metadata, "manager"); err != nil {
		return nil, err
	}
	if cached, ok := s.idempotent(ctx, "add-segment", command.IdempotencyKey); ok {
		return cached, nil
	}
	batch, err := s.repo.Get(ctx, command.BatchID)
	if err != nil {
		return nil, err
	}
	if err := batch.CheckVersion(command.ExpectedVersion); err != nil {
		return nil, err
	}
	segment := domain.RecordingSegment{ID: command.ID, SourceRef: command.SourceRef, StartMillis: command.StartMillis, EndMillis: command.EndMillis, SpeakerCode: command.SpeakerCode, PromptText: command.PromptText}
	if segment.ID == "" {
		segment.ID = s.ids("segment")
	}
	if err := batch.AddSegment(segment); err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, command.ExpectedVersion, "add-segment", command.IdempotencyKey)
}

func (s *Service) UpdateSpecification(ctx context.Context, command SpecificationCommand) (*domain.ReleaseBatch, error) {
	if err := authorize(command.Metadata, "manager"); err != nil {
		return nil, err
	}
	if cached, ok := s.idempotent(ctx, "update-specification", command.IdempotencyKey); ok {
		return cached, nil
	}
	batch, err := s.repo.Get(ctx, command.BatchID)
	if err != nil {
		return nil, err
	}
	if err := batch.CheckVersion(command.ExpectedVersion); err != nil {
		return nil, err
	}
	if err := batch.UpdateSpecification(command.DialectSite, command.PhoneticSystem, command.AllowedLabels, command.MinimumSegmentMillis, command.RequireDual); err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, command.ExpectedVersion, "update-specification", command.IdempotencyKey)
}

func (s *Service) RemoveSegment(ctx context.Context, command RemoveSegmentCommand) (*domain.ReleaseBatch, error) {
	if err := authorize(command.Metadata, "manager"); err != nil {
		return nil, err
	}
	if cached, ok := s.idempotent(ctx, "remove-segment", command.IdempotencyKey); ok {
		return cached, nil
	}
	batch, err := s.repo.Get(ctx, command.BatchID)
	if err != nil {
		return nil, err
	}
	if err := batch.CheckVersion(command.ExpectedVersion); err != nil {
		return nil, err
	}
	if err := batch.RemoveSegment(command.SegmentID); err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, command.ExpectedVersion, "remove-segment", command.IdempotencyKey)
}

func (s *Service) Freeze(ctx context.Context, command BatchCommand) (*domain.ReleaseBatch, error) {
	if err := authorize(command.Metadata, "manager"); err != nil {
		return nil, err
	}
	if cached, ok := s.idempotent(ctx, "freeze", command.IdempotencyKey); ok {
		return cached, nil
	}
	batch, err := s.repo.Get(ctx, command.BatchID)
	if err != nil {
		return nil, err
	}
	if err := batch.CheckVersion(command.ExpectedVersion); err != nil {
		return nil, err
	}
	if err := batch.Freeze(); err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, command.ExpectedVersion, "freeze", command.IdempotencyKey)
}

func (s *Service) Assign(ctx context.Context, command AssignmentCommand) (*domain.ReleaseBatch, error) {
	if err := authorize(command.Metadata, "manager"); err != nil {
		return nil, err
	}
	if cached, ok := s.idempotent(ctx, "assign", command.IdempotencyKey); ok {
		return cached, nil
	}
	batch, err := s.repo.Get(ctx, command.BatchID)
	if err != nil {
		return nil, err
	}
	if err := batch.CheckVersion(command.ExpectedVersion); err != nil {
		return nil, err
	}
	if err := batch.Assign(command.SegmentID, command.First, command.Second); err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, command.ExpectedVersion, "assign", command.IdempotencyKey)
}
