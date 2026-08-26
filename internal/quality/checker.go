package quality

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"acousticverdictworkbench/internal/domain"
)

type checkerCache struct {
	mu          sync.Mutex
	submissions map[string][]domain.AnnotationSubmission
}

type Checker struct {
	cache *checkerCache
}

func NewChecker() Checker {
	return Checker{cache: &checkerCache{submissions: make(map[string][]domain.AnnotationSubmission)}}
}

func (c Checker) Check(batch *domain.ReviewBatch, now time.Time) domain.QualityReport {
	report := domain.QualityReport{BatchID: batch.ID, BatchVersion: batch.Version, CheckedAt: now.UTC(), ClipCount: len(batch.Clips), Issues: []domain.QualityIssue{}}
	allowed := batch.SpeciesSet()
	for _, clip := range batch.Clips {
		subs := c.submissionsFor(batch, clip.ID)
		if len(subs) != 2 {
			report.Issues = append(report.Issues, issue("clip_coverage", "片段必须恰好有两名标注员完成当前轮提交", batch.ID, clip.ID, "", "", ""))
			continue
		}
		report.CoveredClips++
		for _, sub := range subs {
			report.Issues = append(report.Issues, validateSubmission(batch, clip, sub, allowed)...)
		}
	}
	for _, dispute := range batch.Disputes {
		if !dispute.Superseded && dispute.Kind != domain.DisputeAgreement && dispute.Status == domain.DisputeOpen {
			report.Issues = append(report.Issues, issue("unresolved_dispute", "分歧尚未完成仲裁", batch.ID, dispute.ClipID, "", "", dispute.ID))
		}
		if !dispute.Superseded && dispute.Status == domain.DisputeReturned {
			report.Issues = append(report.Issues, issue("returned_annotation", "返标任务尚未通过重新提交和匹配关闭", batch.ID, dispute.ClipID, "", "", dispute.ID))
		}
	}
	if report.ClipCount > 0 {
		report.CoverageRate = float64(report.CoveredClips) / float64(report.ClipCount)
	}
	sort.Slice(report.Issues, func(i, j int) bool {
		a, b := report.Issues[i], report.Issues[j]
		if a.ClipID != b.ClipID {
			return a.ClipID < b.ClipID
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.SubmissionID != b.SubmissionID {
			return a.SubmissionID < b.SubmissionID
		}
		return a.EventID < b.EventID
	})
	report.Passed = report.ClipCount > 0 && len(report.Issues) == 0 && report.CoveredClips == report.ClipCount
	return report
}

func (c Checker) submissionsFor(batch *domain.ReviewBatch, clipID string) []domain.AnnotationSubmission {
	if c.cache == nil {
		return batch.LatestSubmitted(clipID)
	}
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	if cached, ok := c.cache.submissions[clipID]; ok {
		return cloneSubmissions(cached)
	}
	submissions := batch.LatestSubmitted(clipID)
	c.cache.submissions[clipID] = cloneSubmissions(submissions)
	return submissions
}

func cloneSubmissions(source []domain.AnnotationSubmission) []domain.AnnotationSubmission {
	result := make([]domain.AnnotationSubmission, len(source))
	for i := range source {
		result[i] = source[i]
		result[i].Events = append([]domain.CandidateEvent(nil), source[i].Events...)
	}
	return result
}

func validateSubmission(batch *domain.ReviewBatch, clip domain.AudioClip, sub domain.AnnotationSubmission, allowed map[string]struct{}) []domain.QualityIssue {
	issues := []domain.QualityIssue{}
	for _, event := range sub.Events {
		if event.StartMs < 0 || event.EndMs <= event.StartMs || event.EndMs > clip.DurationMs {
			issues = append(issues, issue("time_out_of_bounds", fmt.Sprintf("事件区间必须位于 0-%dms", clip.DurationMs), batch.ID, clip.ID, sub.ID, event.ID, ""))
		}
		if _, ok := allowed[event.SpeciesCode]; !ok {
			issues = append(issues, issue("unknown_species", "事件物种代码不在允许范围内", batch.ID, clip.ID, sub.ID, event.ID, ""))
		}
		if strings.TrimSpace(event.EvidenceNote) == "" {
			issues = append(issues, issue("evidence_required", "事件缺少证据说明", batch.ID, clip.ID, sub.ID, event.ID, ""))
		}
	}
	for i := range sub.Events {
		for j := i + 1; j < len(sub.Events); j++ {
			a, b := sub.Events[i], sub.Events[j]
			if a.SpeciesCode == b.SpeciesCode && a.StartMs == b.StartMs && a.EndMs == b.EndMs {
				issues = append(issues, issue("duplicate_event", "同一提交包含重复的物种和时间区间", batch.ID, clip.ID, sub.ID, b.ID, ""))
			}
		}
	}
	return issues
}

func issue(code, message, batch, clip, sub, event, dispute string) domain.QualityIssue {
	return domain.QualityIssue{Code: code, Message: message, BatchID: batch, ClipID: clip, SubmissionID: sub, EventID: event, DisputeID: dispute}
}
