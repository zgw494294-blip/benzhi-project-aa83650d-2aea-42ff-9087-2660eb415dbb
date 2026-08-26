package application

import (
	"context"
	"strings"

	"acousticverdictworkbench/internal/domain"
)

func (s *Service) Resolve(ctx context.Context, cmd ResolveCommand) (*domain.ReviewBatch, error) {
	if err := authorize(cmd.Metadata, "reviewer"); err != nil {
		return nil, err
	}
	if cmd.ActorID != cmd.ReviewerID {
		return nil, domain.ErrForbidden
	}
	if existing, ok := s.existing(ctx, "dispute.resolve", cmd.Metadata); ok {
		return existing, nil
	}
	batch, err := s.repo.Get(context.WithoutCancel(ctx), cmd.BatchID)
	if err != nil {
		return nil, err
	}
	if cmd.Resolution.NormalizedEvent != nil && cmd.Resolution.NormalizedEvent.ID == "" {
		cmd.Resolution.NormalizedEvent.ID = s.ids("normalized")
	}
	taskID, submissionID := "", ""
	eventIDs := []string(nil)
	if cmd.Resolution.Kind == domain.ResolutionReturn {
		cmd.Resolution.ReturnAnnotator = strings.TrimSpace(cmd.Resolution.ReturnAnnotator)
		taskID, submissionID = s.ids("reannotation"), s.ids("submission")
		latestRound := 0
		for _, submission := range batch.Submissions {
			if submission.ClipID == batchClipID(batch, cmd.DisputeID) && submission.AnnotatorID == cmd.Resolution.ReturnAnnotator && submission.Round > latestRound {
				latestRound = submission.Round
				eventIDs = make([]string, len(submission.Events))
				for i := range eventIDs {
					eventIDs[i] = s.ids("event")
				}
			}
		}
	}
	if err := batch.ResolveDisputeWithTask(cmd.DisputeID, cmd.ReviewerID, cmd.Resolution, s.ids("adjudication"), taskID, submissionID, eventIDs, s.now()); err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, cmd.ExpectedVersion, "dispute.resolve", cmd.IdempotencyKey)
}

func batchClipID(batch *domain.ReviewBatch, disputeID string) string {
	for _, dispute := range batch.Disputes {
		if dispute.ID == disputeID {
			return dispute.ClipID
		}
	}
	return ""
}
