package quality

import (
	"fmt"
	"sort"

	"acousticverdictworkbench/internal/domain"
)

type Matcher struct{ Threshold float64 }

func NewMatcher() Matcher { return Matcher{Threshold: 0.5} }

type pair struct {
	left, right    int
	score          float64
	overlap, union int64
}

func (m Matcher) Match(clipID string, left, right domain.AnnotationSubmission, id func(string) string) []domain.DisputeCase {
	pairs := make([]pair, 0)
	for li, le := range left.Events {
		for ri, re := range right.Events {
			overlap, union, score := intervalIoU(le.StartMs, le.EndMs, re.StartMs, re.EndMs)
			if overlap > 0 {
				pairs = append(pairs, pair{left: li, right: ri, score: score, overlap: overlap, union: union})
			}
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].score != pairs[j].score {
			return pairs[i].score > pairs[j].score
		}
		li, lj := left.Events[pairs[i].left], left.Events[pairs[j].left]
		if li.StartMs != lj.StartMs {
			return li.StartMs < lj.StartMs
		}
		ri, rj := right.Events[pairs[i].right], right.Events[pairs[j].right]
		if ri.StartMs != rj.StartMs {
			return ri.StartMs < rj.StartMs
		}
		if li.ID != lj.ID {
			return li.ID < lj.ID
		}
		return ri.ID < rj.ID
	})
	usedL, usedR := map[int]bool{}, map[int]bool{}
	result := make([]domain.DisputeCase, 0, len(left.Events)+len(right.Events))
	for _, p := range pairs {
		if usedL[p.left] || usedR[p.right] || p.score < m.Threshold {
			continue
		}
		usedL[p.left], usedR[p.right] = true, true
		le, re := left.Events[p.left], right.Events[p.right]
		equal := le.SpeciesCode == re.SpeciesCode
		kind := domain.DisputeConflict
		status := domain.DisputeOpen
		if equal {
			kind, status = domain.DisputeAgreement, domain.DisputeResolved
		}
		result = append(result, domain.DisputeCase{ID: id("case"), ClipID: clipID, Kind: kind, LeftEventID: le.ID, RightEventID: re.ID, LeftSubmissionID: left.ID, RightSubmissionID: right.ID, MatchScore: p.score, Basis: domain.MatchBasis{SpeciesEqual: equal, OverlapMs: p.overlap, UnionMs: p.union, TimeIoU: p.score, Explanation: explanation(equal, p.overlap, p.union, p.score)}, Status: status})
	}
	for i, event := range left.Events {
		if !usedL[i] {
			result = append(result, domain.DisputeCase{ID: id("case"), ClipID: clipID, Kind: domain.DisputeLeftOnly, LeftEventID: event.ID, LeftSubmissionID: left.ID, Basis: domain.MatchBasis{SpeciesEqual: false, Explanation: "右方没有达到时间重叠阈值的候选事件"}, Status: domain.DisputeOpen})
		}
	}
	for i, event := range right.Events {
		if !usedR[i] {
			result = append(result, domain.DisputeCase{ID: id("case"), ClipID: clipID, Kind: domain.DisputeRightOnly, RightEventID: event.ID, RightSubmissionID: right.ID, Basis: domain.MatchBasis{SpeciesEqual: false, Explanation: "左方没有达到时间重叠阈值的候选事件"}, Status: domain.DisputeOpen})
		}
	}
	sortCases(result)
	return result
}

func intervalIoU(a0, a1, b0, b1 int64) (int64, int64, float64) {
	start, end := max(a0, b0), min(a1, b1)
	if end <= start {
		return 0, (a1 - a0) + (b1 - b0), 0
	}
	overlap := end - start
	union := (a1 - a0) + (b1 - b0) - overlap
	return overlap, union, float64(overlap) / float64(union)
}

func explanation(equal bool, overlap, union int64, score float64) string {
	species := "物种代码不同"
	if equal {
		species = "物种代码相同"
	}
	return fmt.Sprintf("%s；时间交集 %dms，并集 %dms，IoU %.4f", species, overlap, union, score)
}

func sortCases(cases []domain.DisputeCase) {
	sort.Slice(cases, func(i, j int) bool {
		if cases[i].Kind != cases[j].Kind {
			return cases[i].Kind < cases[j].Kind
		}
		if cases[i].LeftEventID != cases[j].LeftEventID {
			return cases[i].LeftEventID < cases[j].LeftEventID
		}
		if cases[i].RightEventID != cases[j].RightEventID {
			return cases[i].RightEventID < cases[j].RightEventID
		}
		return cases[i].ID < cases[j].ID
	})
}
