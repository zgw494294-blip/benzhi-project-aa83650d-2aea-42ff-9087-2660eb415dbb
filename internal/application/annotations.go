package application

import (
	"context"

	"acousticverdictworkbench/internal/domain"
)

func (s *Service) SaveDraft(ctx context.Context, cmd DraftCommand) (*domain.ReviewBatch, error) {
	if err := authorize(cmd.Metadata, "annotator"); err != nil {
		return nil, err
	}
	if cmd.ActorID != cmd.AnnotatorID {
		return nil, domain.ErrForbidden
	}
	if len(cmd.Events) > 200 {
		return nil, domain.Invalid("events", "单个草稿最多包含 200 条候选事件")
	}
	if existing, ok := s.existing(ctx, "annotation.draft", cmd.Metadata); ok {
		return existing, nil
	}
	batch, err := s.repo.Get(context.WithoutCancel(ctx), cmd.BatchID)
	if err != nil {
		return nil, err
	}
	if cmd.SubmissionID == "" {
		cmd.SubmissionID = s.ids("submission")
	}
	for i := range cmd.Events {
		if cmd.Events[i].ID == "" {
			cmd.Events[i].ID = s.ids("event")
		}
	}
	if err := batch.SaveDraft(cmd.SubmissionID, cmd.ClipID, cmd.AnnotatorID, cmd.Round, cmd.Events, cmd.RevisionReason, s.now()); err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, cmd.ExpectedVersion, "annotation.draft", cmd.IdempotencyKey)
}

func (s *Service) Submit(ctx context.Context, cmd SubmitCommand) (*domain.ReviewBatch, error) {
	if err := authorize(cmd.Metadata, "annotator"); err != nil {
		return nil, err
	}
	if cmd.ActorID != cmd.AnnotatorID {
		return nil, domain.ErrForbidden
	}
	if existing, ok := s.existing(ctx, "annotation.submit", cmd.Metadata); ok {
		return existing, nil
	}
	batch, err := s.repo.Get(context.WithoutCancel(ctx), cmd.BatchID)
	if err != nil {
		return nil, err
	}
	if err := batch.SubmitConfirmed(cmd.ClipID, cmd.AnnotatorID, cmd.Round, cmd.RevisionReason, cmd.Confirmed, s.now()); err != nil {
		return nil, err
	}
	submissions := batch.LatestSubmitted(cmd.ClipID)
	if len(submissions) == 2 {
		cases := s.matcher.Match(cmd.ClipID, submissions[0], submissions[1], s.ids)
		if err := batch.ReplaceClipDisputes(cmd.ClipID, cases, s.now()); err != nil {
			return nil, err
		}
	}
	return s.commit(ctx, batch, cmd.ExpectedVersion, "annotation.submit", cmd.IdempotencyKey)
}
