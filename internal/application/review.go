package application

import (
	"context"

	"phonemereleasedesk/internal/domain"
	"phonemereleasedesk/internal/verification"
)

func (s *Service) Decide(ctx context.Context, command DecisionCommand) (*domain.ReleaseBatch, error) {
	if err := authorize(command.Metadata, "adjudicator"); err != nil {
		return nil, err
	}
	if command.ActorID != command.AdjudicatorID {
		return nil, domain.ErrForbidden
	}
	if cached, ok := s.idempotent(ctx, "decide", command.IdempotencyKey); ok {
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
	if err := batch.Decide(command.SegmentID, command.IntervalKey, command.ResolvedLabel, command.Reason, command.AdjudicatorID, s.now()); err != nil {
		return nil, err
	}
	if wasRepair && batch.RepairsResolved() {
		scope := verification.AffectedSegments(batch.Repairs)
		result := verification.Run(batch, s.ids("check"), scope, s.now())
		if err := batch.RecordVerification(result.Run); err != nil {
			return nil, err
		}
	}
	return s.commit(ctx, batch, command.ExpectedVersion, "decide", command.IdempotencyKey)
}

func (s *Service) RequestRepair(ctx context.Context, command RepairCommand) (*domain.ReleaseBatch, error) {
	if err := authorize(command.Metadata, "reviewer"); err != nil {
		return nil, err
	}
	if command.ActorID != command.ReviewerID {
		return nil, domain.ErrForbidden
	}
	if cached, ok := s.idempotent(ctx, "repair", command.IdempotencyKey); ok {
		return cached, nil
	}
	batch, err := s.repo.Get(ctx, command.BatchID)
	if err != nil {
		return nil, err
	}
	if err := batch.CheckVersion(command.ExpectedVersion); err != nil {
		return nil, err
	}
	request := domain.RepairRequest{SegmentID: command.SegmentID, Rule: command.Rule, TargetKind: command.TargetKind, AnnotatorID: command.AnnotatorID, IntervalKey: command.IntervalKey, Reason: command.Reason, ReviewerID: command.ReviewerID}
	if err := batch.RequestRepair(request, s.now()); err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, command.ExpectedVersion, "repair", command.IdempotencyKey)
}

func (s *Service) Seal(ctx context.Context, command SealCommand) (*domain.ReleaseBatch, error) {
	if err := authorize(command.Metadata, "reviewer"); err != nil {
		return nil, err
	}
	if command.ActorID != command.ReviewerID {
		return nil, domain.ErrForbidden
	}
	if cached, ok := s.idempotent(ctx, "seal", command.IdempotencyKey); ok {
		return cached, nil
	}
	batch, err := s.repo.Get(ctx, command.BatchID)
	if err != nil {
		return nil, err
	}
	if err := batch.CheckVersion(command.ExpectedVersion); err != nil {
		return nil, err
	}
	credential, err := verification.IssueCredential(batch, s.ids("credential"), command.ReviewerID, s.now())
	if err != nil {
		return nil, err
	}
	if err := batch.Seal(credential, command.ReviewerID, s.now()); err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, command.ExpectedVersion, "seal", command.IdempotencyKey)
}

type CredentialVerification struct {
	Valid            bool                        `json:"valid"`
	StoredDigest     string                      `json:"storedDigest"`
	RecomputedDigest string                      `json:"recomputedDigest"`
	Message          string                      `json:"message"`
	Digest           verification.DimensionCheck `json:"digest"`
	SegmentCount     verification.DimensionCheck `json:"segmentCount"`
	IntervalCount    verification.DimensionCheck `json:"intervalCount"`
}

func (s *Service) VerifyCredential(ctx context.Context, credentialID string) (CredentialVerification, error) {
	batch, err := s.findCredentialBatch(ctx, credentialID, "")
	if err != nil {
		return CredentialVerification{}, err
	}
	checks, err := verification.VerifyCredentialDimensions(batch, *batch.Credential)
	if err != nil {
		return CredentialVerification{}, err
	}
	message := "发布凭据有效，规范化摘要及计数一致"
	if !checks.Valid {
		message = "发布凭据无效，摘要或计数不一致"
	}
	recomputed, _ := checks.Digest.Recomputed.(string)
	return CredentialVerification{Valid: checks.Valid, StoredDigest: batch.Credential.ManifestDigest, RecomputedDigest: recomputed, Message: message, Digest: checks.Digest, SegmentCount: checks.SegmentCount, IntervalCount: checks.IntervalCount}, nil
}
