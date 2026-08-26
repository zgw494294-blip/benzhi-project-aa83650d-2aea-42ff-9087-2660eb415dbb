package application

import (
	"context"

	"phonemereleasedesk/internal/domain"
	"phonemereleasedesk/internal/verification"
)

func (s *Service) SaveSubmission(ctx context.Context, command SubmissionCommand) (*domain.ReleaseBatch, error) {
	if err := authorize(command.Metadata, "annotator"); err != nil {
		return nil, err
	}
	if command.ActorID != command.AnnotatorID {
		return nil, domain.ErrForbidden
	}
	if cached, ok := s.idempotent(ctx, "submission", command.IdempotencyKey); ok {
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
	if err := batch.SaveSubmission(command.SegmentID, command.AnnotatorID, command.Intervals, command.Submit, s.now()); err != nil {
		return nil, err
	}
	if wasRepair && command.Submit && batch.RepairsResolved() {
		scope := verification.AffectedSegments(batch.Repairs)
		result := verification.Run(batch, s.ids("check"), scope, s.now())
		if err := batch.RecordVerification(result.Run); err != nil {
			return nil, err
		}
	}
	return s.commit(ctx, batch, command.ExpectedVersion, "submission", command.IdempotencyKey)
}

func (s *Service) ViewOwnSubmission(ctx context.Context, batchID, segmentID, actor string) (*domain.AnnotationSubmission, error) {
	batch, err := s.repo.Get(ctx, batchID)
	if err != nil {
		return nil, err
	}
	return batch.VisibleSubmission(segmentID, actor)
}

func (s *Service) RunCheck(ctx context.Context, command BatchCommand) (*domain.ReleaseBatch, error) {
	if err := authorize(command.Metadata, "manager", "reviewer"); err != nil {
		return nil, err
	}
	if cached, ok := s.idempotent(ctx, "check", command.IdempotencyKey); ok {
		return cached, nil
	}
	batch, err := s.repo.Get(ctx, command.BatchID)
	if err != nil {
		return nil, err
	}
	if err := batch.CheckVersion(command.ExpectedVersion); err != nil {
		return nil, err
	}
	if batch.State == domain.StateRepair {
		if !batch.RepairsResolved() {
			return nil, domain.Invalid("repairs", "完成命中项修正后才能重检")
		}
		scope := verification.AffectedSegments(batch.Repairs)
		result := verification.Run(batch, s.ids("check"), scope, s.now())
		if err := batch.RecordVerification(result.Run); err != nil {
			return nil, err
		}
	} else {
		if err := batch.BeginChecking(); err != nil {
			return nil, err
		}
		result := verification.Run(batch, s.ids("check"), nil, s.now())
		if err := batch.RecordVerification(result.Run); err != nil {
			return nil, err
		}
		if err := batch.InstallDecisions(result.Conflicts); err != nil {
			return nil, err
		}
	}
	return s.commit(ctx, batch, command.ExpectedVersion, "check", command.IdempotencyKey)
}
