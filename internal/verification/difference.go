package verification

import (
	"fmt"
	"sort"

	"phonemereleasedesk/internal/domain"
)

func findingKey(f domain.Finding) string {
	return fmt.Sprintf("%s|%s|%s|%s", f.Rule, f.SegmentID, f.ActorID, f.Interval)
}

func Difference(before, after []domain.Finding) []string {
	oldSet, newSet := map[string]bool{}, map[string]bool{}
	for _, finding := range before {
		oldSet[findingKey(finding)] = true
	}
	for _, finding := range after {
		newSet[findingKey(finding)] = true
	}
	result := []string{}
	for key := range oldSet {
		if !newSet[key] {
			result = append(result, "已消除:"+key)
		}
	}
	for key := range newSet {
		if !oldSet[key] {
			result = append(result, "新增:"+key)
		}
	}
	sort.Strings(result)
	return result
}

type FindingChange struct {
	Status  string         `json:"status"`
	Finding domain.Finding `json:"finding"`
	Note    string         `json:"note,omitempty"`
}

type ChangeCount struct {
	Key        string `json:"key"`
	Added      int    `json:"added"`
	Persisting int    `json:"persisting"`
	Resolved   int    `json:"resolved"`
}

type RunComparison struct {
	BeforeRunID string          `json:"beforeRunId"`
	AfterRunID  string          `json:"afterRunId"`
	Scope       []string        `json:"scope,omitempty"`
	Changes     []FindingChange `json:"changes"`
	ByRule      []ChangeCount   `json:"byRule"`
	BySegment   []ChangeCount   `json:"bySegment"`
	Added       int             `json:"added"`
	Persisting  int             `json:"persisting"`
	Resolved    int             `json:"resolved"`
}

func CompareRuns(before, after domain.VerificationRun) RunComparison {
	oldSet, newSet := map[string]domain.Finding{}, map[string]domain.Finding{}
	for _, finding := range before.Findings {
		oldSet[findingKey(finding)] = finding
	}
	for _, finding := range after.Findings {
		newSet[findingKey(finding)] = finding
	}
	scoped := map[string]bool{}
	for _, segmentID := range after.Scope {
		scoped[segmentID] = true
	}
	targeted := len(after.Scope) > 0
	beforeScoped := map[string]bool{}
	for _, segmentID := range before.Scope {
		beforeScoped[segmentID] = true
	}
	beforeTargeted := len(before.Scope) > 0
	changes := []FindingChange{}
	for key, finding := range oldSet {
		if next, exists := newSet[key]; exists {
			changes = append(changes, FindingChange{Status: "persisting", Finding: next})
			continue
		}
		if targeted && !scoped[finding.SegmentID] {
			changes = append(changes, FindingChange{Status: "persisting", Finding: finding, Note: "该片段不在后一次定向重检范围内，保留原状态"})
			continue
		}
		changes = append(changes, FindingChange{Status: "resolved", Finding: finding})
	}
	for key, finding := range newSet {
		if _, exists := oldSet[key]; !exists {
			if beforeTargeted && !beforeScoped[finding.SegmentID] {
				changes = append(changes, FindingChange{Status: "persisting", Finding: finding, Note: "该片段不在前一次定向检查范围内，不能判定为新增"})
			} else {
				changes = append(changes, FindingChange{Status: "added", Finding: finding})
			}
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		a, b := findingKey(changes[i].Finding), findingKey(changes[j].Finding)
		if a != b {
			return a < b
		}
		return changes[i].Status < changes[j].Status
	})
	result := RunComparison{BeforeRunID: before.ID, AfterRunID: after.ID, Scope: append([]string(nil), after.Scope...), Changes: changes}
	rules, segments := map[string]*ChangeCount{}, map[string]*ChangeCount{}
	for _, change := range changes {
		incrementGroup(rules, change.Finding.Rule, change.Status)
		incrementGroup(segments, change.Finding.SegmentID, change.Status)
		switch change.Status {
		case "added":
			result.Added++
		case "persisting":
			result.Persisting++
		case "resolved":
			result.Resolved++
		}
	}
	result.ByRule = orderedCounts(rules)
	result.BySegment = orderedCounts(segments)
	return result
}

func incrementChange(count *ChangeCount, status string) {
	switch status {
	case "added":
		count.Added++
	case "persisting":
		count.Persisting++
	case "resolved":
		count.Resolved++
	}
}
func incrementGroup(groups map[string]*ChangeCount, key, status string) {
	if _, ok := groups[key]; !ok {
		groups[key] = &ChangeCount{Key: key}
	}
	incrementChange(groups[key], status)
}
func orderedCounts(input map[string]*ChangeCount) []ChangeCount {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]ChangeCount, 0, len(keys))
	for _, key := range keys {
		result = append(result, *input[key])
	}
	return result
}

func AffectedSegments(repairs []domain.RepairRequest) []string {
	activeRound := ""
	for i := len(repairs) - 1; i >= 0; i-- {
		if repairs[i].RecheckRunID == "" {
			activeRound = repairs[i].RoundID
			break
		}
	}
	seen := map[string]bool{}
	for _, repair := range repairs {
		if repair.RoundID == activeRound && repair.RecheckRunID == "" && repair.ResolvedAt != nil {
			seen[repair.SegmentID] = true
		}
	}
	result := make([]string, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}
