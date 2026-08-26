package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func (b *ReleaseBatch) RequestRepair(request RepairRequest, now time.Time) error {
	if err := b.EnsureWritable(); err != nil {
		return err
	}
	if b.State != StateCandidate && b.State != StateRepair {
		return ErrInvalidState
	}
	if _, exists := b.Segments[request.SegmentID]; !exists {
		return ErrNotFound
	}
	if strings.TrimSpace(request.Rule) == "" || strings.TrimSpace(request.Reason) == "" || strings.TrimSpace(request.ReviewerID) == "" {
		return Invalid("repair", "规则、原因和复核员均为必填")
	}
	if len(b.VerificationRuns) == 0 {
		return Invalid("rule", "没有可用于定向返修的检查运行")
	}
	ruleMatched := false
	for _, finding := range b.EffectiveFindings() {
		if finding.SegmentID == request.SegmentID && finding.Rule == request.Rule {
			ruleMatched = true
			break
		}
	}
	if !ruleMatched {
		return Invalid("rule", "返修规则未命中该片段的当前有效检查结果")
	}
	for _, current := range b.Repairs {
		if current.RecheckRunID == "" && current.SegmentID == request.SegmentID && current.TargetKind == request.TargetKind && current.AnnotatorID == request.AnnotatorID && current.IntervalKey == request.IntervalKey {
			return Invalid("repair", "同一命中目标已有未完成返修任务")
		}
	}
	request.ID = fmt.Sprintf("repair-%s-%d", request.SegmentID, len(b.Repairs)+1)
	request.CreatedAt = now.UTC()
	request.Status = "pending_correction"
	if len(b.VerificationRuns) > 0 {
		request.PreviousRunID = b.VerificationRuns[len(b.VerificationRuns)-1].ID
	}
	request.RoundID = activeRepairRound(b)
	if request.RoundID == "" {
		request.RoundID = fmt.Sprintf("repair-round-%d", len(b.Repairs)+1)
	}
	switch request.TargetKind {
	case "annotation":
		list := b.Submissions[request.SegmentID]
		found := false
		for i := range list {
			if list[i].AnnotatorID == request.AnnotatorID {
				list[i].Status = SubmissionUnlocked
				found = true
			}
		}
		if !found {
			return ErrNotFound
		}
		b.Submissions[request.SegmentID] = list
	case "adjudication":
		key := DecisionKey(request.SegmentID, request.IntervalKey)
		decision, found := b.Decisions[key]
		if !found {
			return ErrNotFound
		}
		decision.Unlocked = true
		b.Decisions[key] = decision
	default:
		return Invalid("targetKind", "返修目标必须是 annotation 或 adjudication")
	}
	b.Repairs = append(b.Repairs, request)
	b.State = StateRepair
	b.Version++
	return nil
}

func activeRepairRound(b *ReleaseBatch) string {
	if b.State != StateRepair {
		return ""
	}
	for i := len(b.Repairs) - 1; i >= 0; i-- {
		if b.Repairs[i].RecheckRunID == "" {
			return b.Repairs[i].RoundID
		}
	}
	return ""
}

func (b *ReleaseBatch) resolveRepairs(kind, segmentID, annotator, interval string, now time.Time) {
	instant := now.UTC()
	for i := range b.Repairs {
		request := &b.Repairs[i]
		if request.ResolvedAt != nil || request.TargetKind != kind || request.SegmentID != segmentID {
			continue
		}
		if kind == "annotation" && request.AnnotatorID == annotator {
			request.ResolvedAt = &instant
			request.Status = "submitted_pending_recheck"
		}
		if kind == "adjudication" && request.IntervalKey == interval {
			request.ResolvedAt = &instant
			request.Status = "submitted_pending_recheck"
		}
	}
}

func (b *ReleaseBatch) RepairsResolved() bool {
	round := activeRepairRound(b)
	if round == "" {
		return true
	}
	for _, request := range b.Repairs {
		if request.RoundID == round && request.RecheckRunID == "" && request.ResolvedAt == nil {
			return false
		}
	}
	return true
}

func (b *ReleaseBatch) RecordVerification(run VerificationRun) error {
	if b.State != StateChecking && b.State != StateRepair {
		return ErrInvalidState
	}
	if b.State == StateRepair && !b.RepairsResolved() {
		return Invalid("repairs", "仍有未完成的定向返修")
	}
	b.VerificationRuns = append(b.VerificationRuns, run)
	if b.State == StateRepair {
		b.finishRepairRound(run)
	}
	b.Version++
	return nil
}

func (b *ReleaseBatch) finishRepairRound(run VerificationRun) {
	round := activeRepairRound(b)
	failedTasks := map[string]bool{}
	for i := range b.Repairs {
		task := &b.Repairs[i]
		if task.RoundID != round || task.RecheckRunID != "" {
			continue
		}
		task.RecheckRunID = run.ID
		var previous *VerificationRun
		for j := range b.VerificationRuns {
			if b.VerificationRuns[j].ID == task.PreviousRunID {
				previous = &b.VerificationRuns[j]
				break
			}
		}
		task.Difference = differenceForSegment(previous, run, task.SegmentID)
		taskPassed := true
		for _, finding := range run.Findings {
			if finding.SegmentID == task.SegmentID && finding.Severity == "error" {
				taskPassed = false
				break
			}
		}
		if taskPassed {
			task.Status = "recheck_passed"
		} else {
			task.Status = "recheck_failed"
			failedTasks[task.ID] = true
		}
	}
	if run.Passed {
		if b.allDecided() {
			b.State = StateCandidate
		} else {
			b.State = StateAdjudicating
		}
		return
	}
	// 未通过时开启同一命中范围的下一轮修正，保留旧任务和差异轨迹只读可查。
	retryRound := fmt.Sprintf("%s-retry-%d", round, len(b.Repairs)+1)
	for _, previous := range append([]RepairRequest(nil), b.Repairs...) {
		if previous.RoundID != round || previous.RecheckRunID != run.ID || !failedTasks[previous.ID] {
			continue
		}
		next := previous
		next.ID = fmt.Sprintf("repair-%s-%d", previous.SegmentID, len(b.Repairs)+1)
		next.RoundID = retryRound
		next.Status = "pending_correction"
		next.CreatedAt = run.CreatedAt
		next.ResolvedAt = nil
		next.PreviousRunID = run.ID
		next.RecheckRunID = ""
		next.Difference = nil
		if previous.TargetKind == "annotation" {
			list := b.Submissions[previous.SegmentID]
			for i := range list {
				if list[i].AnnotatorID == previous.AnnotatorID {
					list[i].Status = SubmissionUnlocked
				}
			}
			b.Submissions[previous.SegmentID] = list
		} else {
			key := DecisionKey(previous.SegmentID, previous.IntervalKey)
			decision := b.Decisions[key]
			decision.Unlocked = true
			b.Decisions[key] = decision
		}
		b.Repairs = append(b.Repairs, next)
	}
}

func differenceForSegment(previous *VerificationRun, run VerificationRun, segmentID string) []FindingDifference {
	oldSet, newSet := map[string]Finding{}, map[string]Finding{}
	key := func(f Finding) string { return f.Rule + "|" + f.SegmentID + "|" + f.ActorID + "|" + f.Interval }
	if previous != nil {
		for _, finding := range previous.Findings {
			if finding.SegmentID == segmentID {
				oldSet[key(finding)] = finding
			}
		}
	}
	for _, finding := range run.Findings {
		if finding.SegmentID == segmentID {
			newSet[key(finding)] = finding
		}
	}
	result := []FindingDifference{}
	for itemKey, finding := range oldSet {
		if current, ok := newSet[itemKey]; ok {
			result = append(result, FindingDifference{Status: "still_exists", Finding: current})
		} else {
			result = append(result, FindingDifference{Status: "resolved", Finding: finding})
		}
	}
	for itemKey, finding := range newSet {
		if _, ok := oldSet[itemKey]; !ok {
			result = append(result, FindingDifference{Status: "added", Finding: finding})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := key(result[i].Finding), key(result[j].Finding)
		if left != right {
			return left < right
		}
		return result[i].Status < result[j].Status
	})
	return result
}

// EffectiveFindings 将定向重检视为只替换 scope 内的结果，未重检片段继续保留最近状态。
func (b *ReleaseBatch) EffectiveFindings() []Finding {
	effective := map[string]Finding{}
	key := func(f Finding) string { return f.Rule + "|" + f.SegmentID + "|" + f.ActorID + "|" + f.Interval }
	for _, run := range b.VerificationRuns {
		if len(run.Scope) == 0 {
			effective = map[string]Finding{}
		} else {
			scoped := map[string]bool{}
			for _, segmentID := range run.Scope {
				scoped[segmentID] = true
			}
			for itemKey, finding := range effective {
				if scoped[finding.SegmentID] {
					delete(effective, itemKey)
				}
			}
		}
		for _, finding := range run.Findings {
			effective[key(finding)] = finding
		}
	}
	result := make([]Finding, 0, len(effective))
	for _, finding := range effective {
		result = append(result, finding)
	}
	sort.Slice(result, func(i, j int) bool { return key(result[i]) < key(result[j]) })
	return result
}

func (b *ReleaseBatch) EffectiveVerificationPassed() bool {
	if len(b.VerificationRuns) == 0 {
		return false
	}
	for _, finding := range b.EffectiveFindings() {
		if finding.Severity == "error" {
			return false
		}
	}
	return true
}

func (b *ReleaseBatch) Seal(credential ReleaseCredential, reviewer string, now time.Time) error {
	if err := b.EnsureWritable(); err != nil {
		return err
	}
	if b.State != StateCandidate {
		return ErrInvalidState
	}
	if strings.TrimSpace(reviewer) == "" {
		return Invalid("reviewerId", "复核员不能为空")
	}
	if !b.EffectiveVerificationPassed() {
		return Invalid("verification", "完整范围的有效检查结果必须通过，未重检片段的问题仍会保留")
	}
	instant := now.UTC()
	credential.BatchID = b.ID
	credential.ReviewerID = reviewer
	credential.IssuedAt = instant
	b.Credential = &credential
	b.SealedAt = &instant
	b.State = StateSealed
	b.Version++
	return nil
}
