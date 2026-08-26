package domain

import (
	"fmt"
	"strings"
	"time"
)

func IntervalKey(start, end int64) string { return fmt.Sprintf("%d-%d", start, end) }

func DecisionKey(segmentID, intervalKey string) string { return segmentID + ":" + intervalKey }

func (b *ReleaseBatch) InstallDecisions(decisions []AdjudicationDecision) error {
	if b.State != StateChecking {
		return ErrInvalidState
	}
	b.Decisions = map[string]AdjudicationDecision{}
	for _, decision := range decisions {
		decision.BatchID = b.ID
		decision.ID = "adj-" + decision.SegmentID + "-" + decision.IntervalKey
		b.Decisions[DecisionKey(decision.SegmentID, decision.IntervalKey)] = decision
	}
	if len(decisions) == 0 {
		b.State = StateCandidate
	} else {
		b.State = StateAdjudicating
	}
	b.Version++
	return nil
}

func (b *ReleaseBatch) Decide(segmentID, intervalKey, resolved, reason, adjudicator string, now time.Time) error {
	if err := b.EnsureWritable(); err != nil {
		return err
	}
	if b.State != StateAdjudicating && b.State != StateRepair {
		return ErrInvalidState
	}
	key := DecisionKey(segmentID, intervalKey)
	decision, exists := b.Decisions[key]
	if !exists {
		return ErrNotFound
	}
	if b.State == StateRepair && !decision.Unlocked {
		return Invalid("decision", "该裁定项未被定向解锁")
	}
	resolved, reason, adjudicator = strings.TrimSpace(resolved), strings.TrimSpace(reason), strings.TrimSpace(adjudicator)
	if resolved == "" || reason == "" || adjudicator == "" {
		return Invalid("decision", "裁定标签、理由和裁定员均为必填")
	}
	if !b.HasLabel(resolved) {
		return Invalid("resolvedLabel", "裁定标签不在白名单中")
	}
	instant := now.UTC()
	decision.ResolvedLabel, decision.Reason, decision.AdjudicatorID, decision.DecidedAt, decision.Unlocked = resolved, reason, adjudicator, &instant, false
	b.Decisions[key] = decision
	b.resolveRepairs("adjudication", segmentID, "", intervalKey, now)
	if b.allDecided() {
		b.State = StateCandidate
	}
	b.Version++
	return nil
}

func (b *ReleaseBatch) allDecided() bool {
	for _, item := range b.Decisions {
		if item.ResolvedLabel == "" || item.DecidedAt == nil {
			return false
		}
	}
	return true
}

func (b *ReleaseBatch) CandidateReady() bool { return b.State == StateCandidate && b.allDecided() }
