package verification

import (
	"fmt"
	"sort"
	"time"

	"phonemereleasedesk/internal/domain"
)

const RuleSetVersion = "phoneme-release-rules/1.0"

type Result struct {
	Run       domain.VerificationRun
	Conflicts []domain.AdjudicationDecision
}

func Run(batch *domain.ReleaseBatch, id string, scope []string, now time.Time) Result {
	findings := make([]domain.Finding, 0)
	conflicts := make([]domain.AdjudicationDecision, 0)
	selected := selectedSegments(batch, scope)
	matched, compared := 0, 0
	for _, segment := range selected {
		findings = append(findings, checkMetadata(batch, segment)...)
		submissions := batch.Submissions[segment.ID]
		if len(submissions) != 2 {
			findings = append(findings, domain.Finding{Rule: "dual-submission", SegmentID: segment.ID, Severity: "error", Message: "片段没有两份独立提交"})
			continue
		}
		if submissions[0].AnnotatorID == submissions[1].AnnotatorID {
			findings = append(findings, domain.Finding{Rule: "annotator-isolation", SegmentID: segment.ID, Severity: "error", Message: "双份标注来自同一人员"})
		}
		for _, submission := range submissions {
			findings = append(findings, checkIntervals(batch, segment, submission)...)
		}
		left, right := intervalMap(submissions[0].Intervals), intervalMap(submissions[1].Intervals)
		keys := unionKeys(left, right)
		for _, key := range keys {
			compared++
			first, firstOK := left[key]
			second, secondOK := right[key]
			if firstOK && secondOK && first == second {
				matched++
				continue
			}
			labels := uniqueLabels(first, firstOK, second, secondOK)
			conflicts = append(conflicts, domain.AdjudicationDecision{SegmentID: segment.ID, IntervalKey: key, CandidateLabels: labels})
			findings = append(findings, domain.Finding{Rule: "dual-agreement", SegmentID: segment.ID, Interval: key, Severity: "conflict", Message: fmt.Sprintf("双标不一致：%v", labels)})
		}
	}
	agreement := 1.0
	if compared > 0 {
		agreement = float64(matched) / float64(compared)
	}
	passed := true
	for _, finding := range findings {
		if finding.Severity == "error" {
			passed = false
			break
		}
	}
	run := domain.VerificationRun{ID: id, BatchID: batch.ID, BatchVersion: batch.Version, RuleSetVersion: RuleSetVersion, Scope: append([]string(nil), scope...), Findings: findings, AgreementRate: agreement, Passed: passed, CreatedAt: now.UTC()}
	if len(batch.VerificationRuns) > 0 {
		previous := batch.VerificationRuns[len(batch.VerificationRuns)-1]
		run.PreviousRunID = previous.ID
		comparison := CompareRuns(previous, run)
		for _, change := range comparison.Changes {
			run.Difference = append(run.Difference, change.Status+":"+findingKey(change.Finding))
		}
	}
	return Result{Run: run, Conflicts: conflicts}
}

func selectedSegments(batch *domain.ReleaseBatch, scope []string) []domain.RecordingSegment {
	if len(scope) == 0 {
		return batch.OrderedSegments()
	}
	allowed := map[string]bool{}
	for _, id := range scope {
		allowed[id] = true
	}
	items := make([]domain.RecordingSegment, 0, len(scope))
	for _, item := range batch.OrderedSegments() {
		if allowed[item.ID] {
			items = append(items, item)
		}
	}
	return items
}

func checkMetadata(batch *domain.ReleaseBatch, segment domain.RecordingSegment) []domain.Finding {
	result := []domain.Finding{}
	if segment.SourceRef == "" || segment.SpeakerCode == "" || segment.PromptText == "" {
		result = append(result, domain.Finding{Rule: "segment-metadata", SegmentID: segment.ID, Severity: "error", Message: "片段元数据不完整"})
	}
	if segment.EndMillis <= segment.StartMillis || segment.EndMillis-segment.StartMillis < batch.MinimumSegmentMillis {
		result = append(result, domain.Finding{Rule: "segment-boundary", SegmentID: segment.ID, Severity: "error", Message: "片段边界或时长无效"})
	}
	return result
}

func checkIntervals(batch *domain.ReleaseBatch, segment domain.RecordingSegment, submission domain.AnnotationSubmission) []domain.Finding {
	result := []domain.Finding{}
	if submission.Status != domain.SubmissionSubmitted {
		result = append(result, domain.Finding{Rule: "submission-status", SegmentID: segment.ID, ActorID: submission.AnnotatorID, Severity: "error", Message: "标注尚未独立提交"})
	}
	expected := segment.StartMillis
	for _, interval := range submission.Intervals {
		key := domain.IntervalKey(interval.StartMillis, interval.EndMillis)
		if interval.StartMillis != expected {
			result = append(result, domain.Finding{Rule: "interval-continuity", SegmentID: segment.ID, ActorID: submission.AnnotatorID, Interval: key, Severity: "error", Message: "区间不连续"})
		}
		if interval.EndMillis <= interval.StartMillis || interval.StartMillis < segment.StartMillis || interval.EndMillis > segment.EndMillis {
			result = append(result, domain.Finding{Rule: "interval-boundary", SegmentID: segment.ID, ActorID: submission.AnnotatorID, Interval: key, Severity: "error", Message: "音素区间越界"})
		}
		if !batch.HasLabel(interval.Label) {
			result = append(result, domain.Finding{Rule: "label-whitelist", SegmentID: segment.ID, ActorID: submission.AnnotatorID, Interval: key, Severity: "error", Message: "标签不在允许集合中"})
		}
		expected = interval.EndMillis
	}
	if len(submission.Intervals) == 0 || expected != segment.EndMillis {
		result = append(result, domain.Finding{Rule: "interval-coverage", SegmentID: segment.ID, ActorID: submission.AnnotatorID, Severity: "error", Message: "音素区间未完整覆盖片段"})
	}
	return result
}

func intervalMap(intervals []domain.PhonemeInterval) map[string]string {
	result := make(map[string]string, len(intervals))
	for _, item := range intervals {
		result[domain.IntervalKey(item.StartMillis, item.EndMillis)] = item.Label
	}
	return result
}

func unionKeys(left, right map[string]string) []string {
	seen := map[string]bool{}
	for key := range left {
		seen[key] = true
	}
	for key := range right {
		seen[key] = true
	}
	result := make([]string, 0, len(seen))
	for key := range seen {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func uniqueLabels(first string, firstOK bool, second string, secondOK bool) []string {
	labels := []string{}
	if firstOK {
		labels = append(labels, first)
	}
	if secondOK && (!firstOK || second != first) {
		labels = append(labels, second)
	}
	sort.Strings(labels)
	return labels
}
