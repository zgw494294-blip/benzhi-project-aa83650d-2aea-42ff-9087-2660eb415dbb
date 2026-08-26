package application

import (
	"context"
	"sort"
	"strings"

	"acousticverdictworkbench/internal/domain"
	"acousticverdictworkbench/internal/quality"
)

func (s *Service) ManifestDetails(ctx context.Context, batchID string, meta Metadata, query ManifestEventQuery) (*ManifestDetails, error) {
	if err := authorize(meta, "release_manager", "reviewer"); err != nil {
		return nil, err
	}
	if query.Page < 1 || query.Page > 1000000 || query.PageSize < 1 || query.PageSize > 100 {
		return nil, domain.Invalid("page", "page 必须在 1 到 1000000 之间且 pageSize 必须在 1 到 100 之间")
	}
	query.ClipID = strings.TrimSpace(query.ClipID)
	query.SpeciesCode = strings.ToUpper(strings.TrimSpace(query.SpeciesCode))
	if len(query.ClipID) > 128 {
		return nil, domain.Invalid("clipId", "clipId 不能超过 128 字符")
	}
	if query.SpeciesCode != "" {
		codes, err := domain.NormalizeSpecies([]string{query.SpeciesCode})
		if err != nil {
			return nil, err
		}
		query.SpeciesCode = codes[0]
	}
	if (query.StartMs != nil && *query.StartMs < 0) || (query.EndMs != nil && *query.EndMs < 0) || (query.StartMs != nil && query.EndMs != nil && *query.EndMs < *query.StartMs) {
		return nil, domain.Invalid("timeRange", "时间筛选必须为非负数且结束毫秒不能早于开始毫秒")
	}
	batch, err := s.repo.Get(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if batch.Status != domain.BatchReleased {
		return nil, domain.ErrStateConflict
	}
	if batch.Manifest == nil {
		return nil, domain.Integrity("manifest", "已发布批次缺少不可变发布清单")
	}
	events := make([]domain.NormalizedEvent, 0)
	for _, event := range batch.Manifest.NormalizedEvents {
		if query.ClipID != "" && event.ClipID != query.ClipID {
			continue
		}
		if query.SpeciesCode != "" && event.SpeciesCode != query.SpeciesCode {
			continue
		}
		if query.StartMs != nil && event.EndMs < *query.StartMs {
			continue
		}
		if query.EndMs != nil && event.StartMs > *query.EndMs {
			continue
		}
		events = append(events, event)
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].ClipID != events[j].ClipID {
			return events[i].ClipID < events[j].ClipID
		}
		if events[i].StartMs != events[j].StartMs {
			return events[i].StartMs < events[j].StartMs
		}
		if events[i].EndMs != events[j].EndMs {
			return events[i].EndMs < events[j].EndMs
		}
		return events[i].SpeciesCode < events[j].SpeciesCode
	})
	total := len(events)
	start := (query.Page - 1) * query.PageSize
	if start > total {
		start = total
	}
	end := start + query.PageSize
	if end > total {
		end = total
	}
	return &ManifestDetails{ManifestID: batch.Manifest.ID, BatchID: batch.ID, Events: append([]domain.NormalizedEvent(nil), events[start:end]...), Total: total, Page: query.Page, PageSize: query.PageSize, SourceClips: append([]domain.ClipSummary(nil), batch.Manifest.SourceClips...), AdjudicationTrail: append([]domain.AdjudicationRecord(nil), batch.Manifest.AdjudicationTrail...)}, nil
}

func (s *Service) VerifyManifest(ctx context.Context, batchID string, meta Metadata) (*ManifestVerification, error) {
	if err := authorize(meta, "release_manager", "reviewer"); err != nil {
		return nil, err
	}
	batch, err := s.repo.Get(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if batch.Status != domain.BatchReleased {
		return nil, domain.ErrStateConflict
	}
	if batch.Manifest == nil {
		return nil, domain.Integrity("manifest", "已发布批次缺少不可变发布清单")
	}
	checks, err := quality.VerifyManifest(*batch.Manifest)
	if err != nil {
		return nil, err
	}
	result := &ManifestVerification{ManifestID: batch.Manifest.ID, Consistent: true, Items: make([]DigestVerification, len(checks))}
	for i, check := range checks {
		result.Items[i] = DigestVerification{Field: check.Field, Expected: check.Expected, Actual: check.Actual, Consistent: check.Consistent}
		if !check.Consistent {
			result.Consistent = false
		}
	}
	return result, nil
}

func (s *Service) CheckQuality(ctx context.Context, cmd BatchCommand) (*domain.ReviewBatch, error) {
	if existing, ok := s.existing(ctx, "quality.check", cmd.Metadata); ok {
		return existing, nil
	}
	if err := authorize(cmd.Metadata, "reviewer", "release_manager"); err != nil {
		return nil, err
	}
	batch, err := s.repo.Get(ctx, cmd.BatchID)
	if err != nil {
		return nil, err
	}
	if batch.Status == domain.BatchReleased {
		return nil, domain.ErrAlreadyReleased
	}
	report := s.checker.Check(batch, s.now())
	batch.TouchQuality(report, s.now())
	return s.commit(ctx, batch, cmd.ExpectedVersion, "quality.check", cmd.IdempotencyKey)
}

func (s *Service) Release(ctx context.Context, cmd ReleaseCommand) (*domain.ReviewBatch, error) {
	if existing, ok := s.existing(ctx, "batch.release", cmd.Metadata); ok {
		return existing, nil
	}
	if err := authorize(cmd.Metadata, "release_manager"); err != nil {
		return nil, err
	}
	if cmd.ActorID != cmd.ReleasedBy {
		return nil, domain.ErrForbidden
	}
	batch, err := s.repo.Get(ctx, cmd.BatchID)
	if err != nil {
		return nil, err
	}
	if batch.Status == domain.BatchReleased {
		return nil, domain.ErrAlreadyReleased
	}
	if batch.Version != cmd.ExpectedVersion {
		return nil, domain.ErrVersionConflict
	}
	// 封存入口再次计算，避免使用过期质量结果。
	report := s.checker.Check(batch, s.now())
	if !report.Passed {
		return nil, domain.Invalid("quality", "当前批次仍有阻断项，不能封存")
	}
	batch.TouchQuality(report, s.now())
	manifest, err := quality.BuildManifest(s.ids("manifest"), cmd.ReleasedBy, batch, s.now())
	if err != nil {
		return nil, err
	}
	if err := batch.Seal(manifest, s.now()); err != nil {
		return nil, err
	}
	return s.commit(ctx, batch, cmd.ExpectedVersion, "batch.release", cmd.IdempotencyKey)
}
