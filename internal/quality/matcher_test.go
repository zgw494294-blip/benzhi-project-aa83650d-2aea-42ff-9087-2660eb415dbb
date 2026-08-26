package quality

import (
	"testing"

	"acousticverdictworkbench/internal/domain"
)

func TestMatcherIsDeterministicAndExplainsConflict(t *testing.T) {
	left := domain.AnnotationSubmission{Events: []domain.CandidateEvent{{ID: "l2", SpeciesCode: "BIRD_A", StartMs: 1000, EndMs: 2000}, {ID: "l1", SpeciesCode: "BIRD_A", StartMs: 100, EndMs: 600}}}
	right := domain.AnnotationSubmission{Events: []domain.CandidateEvent{{ID: "r2", SpeciesCode: "BIRD_B", StartMs: 1100, EndMs: 1900}, {ID: "r1", SpeciesCode: "BIRD_A", StartMs: 100, EndMs: 600}}}
	sequence := 0
	ids := func(string) string { sequence++; return string(rune('a' + sequence)) }
	cases := NewMatcher().Match("clip", left, right, ids)
	if len(cases) != 2 {
		t.Fatalf("匹配数=%d", len(cases))
	}
	if cases[0].Kind != domain.DisputeAgreement || cases[1].Kind != domain.DisputeConflict {
		t.Fatalf("分类不稳定：%+v", cases)
	}
	if cases[1].Basis.TimeIoU != 0.8 || cases[1].Basis.SpeciesEqual {
		t.Fatalf("匹配依据错误：%+v", cases[1].Basis)
	}
}
