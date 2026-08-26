package application

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"phonemereleasedesk/internal/domain"
	"phonemereleasedesk/internal/verification"
)

type VerificationRunSummary struct {
	ID             string           `json:"id"`
	RuleSetVersion string           `json:"ruleSetVersion"`
	Scope          []string         `json:"scope,omitempty"`
	ScopeKind      string           `json:"scopeKind"`
	BatchVersion   uint64           `json:"batchVersion"`
	AgreementRate  float64          `json:"agreementRate"`
	Passed         bool             `json:"passed"`
	SeverityCounts map[string]int   `json:"severityCounts"`
	CreatedAt      time.Time        `json:"createdAt"`
	PreviousRunID  string           `json:"previousRunId,omitempty"`
	Findings       []domain.Finding `json:"findings,omitempty"`
}

func (s *Service) VerificationHistory(ctx context.Context, batchID, rule, segmentID string) ([]VerificationRunSummary, error) {
	batch, err := s.repo.Get(ctx, batchID)
	if err != nil {
		return nil, err
	}
	result := make([]VerificationRunSummary, 0, len(batch.VerificationRuns))
	for _, run := range batch.VerificationRuns {
		counts := map[string]int{}
		findings := []domain.Finding{}
		for _, finding := range run.Findings {
			if rule != "" && finding.Rule != rule {
				continue
			}
			if segmentID != "" && finding.SegmentID != segmentID {
				continue
			}
			findings = append(findings, finding)
			counts[finding.Severity]++
		}
		scopeKind := "full"
		if len(run.Scope) > 0 {
			scopeKind = "targeted"
		}
		result = append(result, VerificationRunSummary{ID: run.ID, RuleSetVersion: run.RuleSetVersion, Scope: append([]string(nil), run.Scope...), ScopeKind: scopeKind, BatchVersion: run.BatchVersion, AgreementRate: run.AgreementRate, Passed: run.Passed, SeverityCounts: counts, CreatedAt: run.CreatedAt, PreviousRunID: run.PreviousRunID, Findings: findings})
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].CreatedAt.After(result[j].CreatedAt)
		}
		return result[i].ID > result[j].ID
	})
	return result, nil
}

func (s *Service) CompareVerificationRuns(ctx context.Context, batchID, beforeID, afterID, rule, segmentID string) (verification.RunComparison, error) {
	batch, err := s.repo.Get(ctx, batchID)
	if err != nil {
		return verification.RunComparison{}, err
	}
	var before, after *domain.VerificationRun
	for i := range batch.VerificationRuns {
		run := &batch.VerificationRuns[i]
		if run.ID == beforeID {
			before = run
		}
		if run.ID == afterID {
			after = run
		}
	}
	if before == nil {
		return verification.RunComparison{}, fmt.Errorf("%w: 前一次检查运行不属于当前批次", domain.ErrNotFound)
	}
	if after == nil {
		return verification.RunComparison{}, fmt.Errorf("%w: 后一次检查运行不属于当前批次", domain.ErrNotFound)
	}
	filteredBefore, filteredAfter := *before, *after
	filteredBefore.Findings = filterFindings(before.Findings, rule, segmentID)
	filteredAfter.Findings = filterFindings(after.Findings, rule, segmentID)
	return verification.CompareRuns(filteredBefore, filteredAfter), nil
}

func filterFindings(findings []domain.Finding, rule, segmentID string) []domain.Finding {
	result := []domain.Finding{}
	for _, finding := range findings {
		if rule != "" && finding.Rule != rule {
			continue
		}
		if segmentID != "" && finding.SegmentID != segmentID {
			continue
		}
		result = append(result, finding)
	}
	return result
}

type DecisionQueueItem struct {
	SegmentID       string   `json:"segmentId"`
	IntervalKey     string   `json:"intervalKey"`
	CandidateLabels []string `json:"candidateLabels"`
	ResolvedLabel   string   `json:"resolvedLabel,omitempty"`
	Pending         bool     `json:"pending"`
	RepairUnlocked  bool     `json:"repairUnlocked"`
}

func (s *Service) DecisionQueue(ctx context.Context, batchID, segmentID, candidate string, pendingOnly bool) ([]DecisionQueueItem, error) {
	batch, err := s.repo.Get(ctx, batchID)
	if err != nil {
		return nil, err
	}
	result := []DecisionQueueItem{}
	for _, decision := range batch.Decisions {
		pending := decision.DecidedAt == nil || decision.Unlocked
		if pendingOnly && !pending {
			continue
		}
		if segmentID != "" && decision.SegmentID != segmentID {
			continue
		}
		if candidate != "" && !contains(decision.CandidateLabels, candidate) {
			continue
		}
		result = append(result, DecisionQueueItem{SegmentID: decision.SegmentID, IntervalKey: decision.IntervalKey, CandidateLabels: append([]string(nil), decision.CandidateLabels...), ResolvedLabel: decision.ResolvedLabel, Pending: pending, RepairUnlocked: decision.Unlocked})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].SegmentID != result[j].SegmentID {
			return result[i].SegmentID < result[j].SegmentID
		}
		return result[i].IntervalKey < result[j].IntervalKey
	})
	return result, nil
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

type RepairTaskQuery struct{ SegmentID, Rule, TargetKind, Status, ActorID, Role string }

func (s *Service) RepairTasks(ctx context.Context, batchID string, query RepairTaskQuery) ([]domain.RepairRequest, error) {
	batch, err := s.repo.Get(ctx, batchID)
	if err != nil {
		return nil, err
	}
	result := []domain.RepairRequest{}
	for _, task := range batch.Repairs {
		if query.SegmentID != "" && task.SegmentID != query.SegmentID {
			continue
		}
		if query.Rule != "" && task.Rule != query.Rule {
			continue
		}
		if query.TargetKind != "" && task.TargetKind != query.TargetKind {
			continue
		}
		if query.Status != "" && task.Status != query.Status {
			continue
		}
		switch query.Role {
		case "annotator":
			if task.TargetKind != "annotation" || task.AnnotatorID != query.ActorID {
				continue
			}
		case "adjudicator":
			if task.TargetKind != "adjudication" {
				continue
			}
		case "reviewer", "manager", "":
		default:
			return nil, domain.ErrForbidden
		}
		copyTask := task
		copyTask.Difference = append([]domain.FindingDifference(nil), task.Difference...)
		result = append(result, copyTask)
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].CreatedAt.After(result[j].CreatedAt)
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

type CredentialLookup struct {
	CredentialID string
	BatchID      string
	SegmentID    string
	Page         int
	PageSize     int
}
type CredentialManifestItem struct {
	SegmentID   string `json:"segmentId"`
	SourceRef   string `json:"sourceRef"`
	PromptText  string `json:"promptText"`
	StartMillis int64  `json:"startMillis"`
	EndMillis   int64  `json:"endMillis"`
	Label       string `json:"label"`
}
type CredentialDetail struct {
	Credential domain.ReleaseCredential `json:"credential"`
	Items      []CredentialManifestItem `json:"items"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"pageSize"`
	Total      int                      `json:"total"`
}

func (s *Service) CredentialDetail(ctx context.Context, lookup CredentialLookup) (CredentialDetail, error) {
	batch, err := s.findCredentialBatch(ctx, lookup.CredentialID, lookup.BatchID)
	if err != nil {
		return CredentialDetail{}, err
	}
	page, pageSize := lookup.Page, lookup.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		return CredentialDetail{}, domain.Invalid("pageSize", "每页数量不能超过 100")
	}
	items := []CredentialManifestItem{}
	manifest := append([]domain.ManifestInterval(nil), batch.Credential.Manifest...)
	sort.Slice(manifest, func(i, j int) bool {
		if manifest[i].SegmentID != manifest[j].SegmentID {
			return manifest[i].SegmentID < manifest[j].SegmentID
		}
		if manifest[i].StartMillis != manifest[j].StartMillis {
			return manifest[i].StartMillis < manifest[j].StartMillis
		}
		return manifest[i].EndMillis < manifest[j].EndMillis
	})
	for _, interval := range manifest {
		if lookup.SegmentID != "" && interval.SegmentID != lookup.SegmentID {
			continue
		}
		segment := batch.Segments[interval.SegmentID]
		items = append(items, CredentialManifestItem{SegmentID: interval.SegmentID, SourceRef: segment.SourceRef, PromptText: segment.PromptText, StartMillis: interval.StartMillis, EndMillis: interval.EndMillis, Label: interval.Label})
	}
	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	credential := *batch.Credential
	credential.Manifest = nil
	return CredentialDetail{Credential: credential, Items: items[start:end], Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *Service) VerifyCredentialDetail(ctx context.Context, credentialID, batchID string) (verification.CredentialChecks, error) {
	batch, err := s.findCredentialBatch(ctx, credentialID, batchID)
	if err != nil {
		return verification.CredentialChecks{}, err
	}
	return verification.VerifyCredentialDimensions(batch, *batch.Credential)
}

func (s *Service) findCredentialBatch(ctx context.Context, credentialID, batchID string) (*domain.ReleaseBatch, error) {
	credentialID, batchID = strings.TrimSpace(credentialID), strings.TrimSpace(batchID)
	if credentialID == "" && batchID == "" {
		return nil, domain.Invalid("credential", "credentialId 或 batchId 至少提供一项")
	}
	if batchID != "" {
		batch, err := s.repo.Get(ctx, batchID)
		if err != nil {
			return nil, err
		}
		if batch.State != domain.StateSealed || batch.Credential == nil {
			return nil, fmt.Errorf("%w: 批次尚未封存或尚未签发凭据", domain.ErrInvalidState)
		}
		if credentialID != "" && batch.Credential.ID != credentialID {
			return nil, fmt.Errorf("%w: credentialId 与 batchId 不匹配", domain.ErrNotFound)
		}
		return batch, nil
	}
	batches, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, batch := range batches {
		if batch.State == domain.StateSealed && batch.Credential != nil && batch.Credential.ID == credentialID {
			return batch, nil
		}
	}
	return nil, fmt.Errorf("%w: 发布凭据", domain.ErrNotFound)
}
