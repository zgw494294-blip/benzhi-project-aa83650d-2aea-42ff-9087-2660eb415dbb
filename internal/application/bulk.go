package application

import (
	"context"
	"strings"

	"phonemereleasedesk/internal/domain"
	"phonemereleasedesk/internal/verification"
)

func (s *Service) PreflightSegments(ctx context.Context, command SegmentBatchCommand) (domain.SegmentPreflight, error) {
	if err := authorize(command.Metadata, "manager"); err != nil {
		return domain.SegmentPreflight{}, err
	}
	batch, err := s.repo.Get(ctx, command.BatchID)
	if err != nil {
		return domain.SegmentPreflight{}, err
	}
	if err := batch.CheckVersion(command.ExpectedVersion); err != nil {
		return domain.SegmentPreflight{}, err
	}
	return batch.PreflightSegments(command.Segments)
}

func (s *Service) AddSegments(ctx context.Context, command SegmentBatchCommand) (*domain.ReleaseBatch, error) {
	if err := authorize(command.Metadata, "manager"); err != nil {
		return nil, err
	}
	operation := "bulk-segments:" + command.BatchID
	if strings.TrimSpace(command.IdempotencyKey) == "" {
		return nil, domain.Invalid("Idempotency-Key", "批量确认必须提供幂等键")
	}
	if cached, ok := s.idempotent(ctx, operation, command.IdempotencyKey); ok {
		return cached, nil
	}
	batch, err := s.repo.Get(ctx, command.BatchID)
	if err != nil {
		return nil, err
	}
	if err := batch.CheckVersion(command.ExpectedVersion); err != nil {
		return nil, err
	}
	actual := domain.SegmentContentDigest(command.Segments)
	if strings.TrimSpace(command.ContentDigest) == "" || command.ContentDigest != actual {
		return nil, domain.Invalid("contentDigest", "预检内容摘要与确认内容不一致")
	}
	if err := batch.AddSegments(command.Segments); err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, command.ExpectedVersion, operation, command.IdempotencyKey)
}

func (s *Service) PreviewAssignments(ctx context.Context, command AssignmentBatchCommand) (domain.AssignmentPreview, error) {
	if err := authorize(command.Metadata, "manager"); err != nil {
		return domain.AssignmentPreview{}, err
	}
	batch, err := s.repo.Get(ctx, command.BatchID)
	if err != nil {
		return domain.AssignmentPreview{}, err
	}
	if err := batch.CheckVersion(command.ExpectedVersion); err != nil {
		return domain.AssignmentPreview{}, err
	}
	return batch.PreviewAssignments(command.Plan)
}

func (s *Service) AssignMany(ctx context.Context, command AssignmentBatchCommand) (*domain.ReleaseBatch, error) {
	if err := authorize(command.Metadata, "manager"); err != nil {
		return nil, err
	}
	operation := "bulk-assignments:" + command.BatchID
	if strings.TrimSpace(command.IdempotencyKey) == "" {
		return nil, domain.Invalid("Idempotency-Key", "批量确认必须提供幂等键")
	}
	if cached, ok := s.idempotent(ctx, operation, command.IdempotencyKey); ok {
		return cached, nil
	}
	batch, err := s.repo.Get(ctx, command.BatchID)
	if err != nil {
		return nil, err
	}
	if err := batch.CheckVersion(command.ExpectedVersion); err != nil {
		return nil, err
	}
	if err := batch.AssignMany(command.Plan); err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, command.ExpectedVersion, operation, command.IdempotencyKey)
}

func (s *Service) DecideMany(ctx context.Context, command DecisionBatchCommand) (*domain.ReleaseBatch, error) {
	if err := authorize(command.Metadata, "adjudicator"); err != nil {
		return nil, err
	}
	operation := "bulk-decisions:" + command.BatchID
	if strings.TrimSpace(command.IdempotencyKey) == "" {
		return nil, domain.Invalid("Idempotency-Key", "批量确认必须提供幂等键")
	}
	if cached, ok := s.idempotent(ctx, operation, command.IdempotencyKey); ok {
		return cached, nil
	}
	batch, err := s.repo.Get(ctx, command.BatchID)
	if err != nil {
		return nil, err
	}
	if err := batch.CheckVersion(command.ExpectedVersion); err != nil {
		return nil, err
	}
	wasRepair := batch.State == domain.StateRepair
	if err := batch.DecideMany(command.Decisions, command.ActorID, s.now()); err != nil {
		return nil, err
	}
	if wasRepair && batch.RepairsResolved() {
		scope := verification.AffectedSegments(batch.Repairs)
		result := verification.Run(batch, s.ids("check"), scope, s.now())
		if err := batch.RecordVerification(result.Run); err != nil {
			return nil, err
		}
	}
	return s.commit(ctx, batch, command.ExpectedVersion, operation, command.IdempotencyKey)
}
