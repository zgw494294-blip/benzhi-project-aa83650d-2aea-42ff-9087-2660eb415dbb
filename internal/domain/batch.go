package domain

import (
	"sort"
	"strings"
	"time"
)

type ReleaseBatch struct {
	ID                   string                            `json:"id"`
	DialectSite          string                            `json:"dialectSite"`
	PhoneticSystem       string                            `json:"phoneticSystem"`
	AllowedLabels        []string                          `json:"allowedLabels"`
	MinimumSegmentMillis int64                             `json:"minimumSegmentMillis"`
	RequireDual          bool                              `json:"requireDual"`
	State                BatchState                        `json:"state"`
	Version              uint64                            `json:"version"`
	CreatedAt            time.Time                         `json:"createdAt"`
	SealedAt             *time.Time                        `json:"sealedAt,omitempty"`
	Segments             map[string]RecordingSegment       `json:"segments"`
	Assignments          map[string][]string               `json:"assignments"`
	Submissions          map[string][]AnnotationSubmission `json:"submissions"`
	Decisions            map[string]AdjudicationDecision   `json:"decisions"`
	VerificationRuns     []VerificationRun                 `json:"verificationRuns"`
	Repairs              []RepairRequest                   `json:"repairs"`
	Credential           *ReleaseCredential                `json:"credential,omitempty"`
}

func NewReleaseBatch(id, site, system string, labels []string, minimum int64, requireDual bool, now time.Time) (*ReleaseBatch, error) {
	if strings.TrimSpace(id) == "" {
		return nil, Invalid("id", "批次 ID 不能为空")
	}
	if strings.TrimSpace(site) == "" || strings.TrimSpace(system) == "" {
		return nil, Invalid("specification", "方言点和音标体系不能为空")
	}
	if minimum <= 0 {
		return nil, Invalid("minimumSegmentMillis", "最短片段时长必须大于零")
	}
	normalized, err := NormalizeLabels(labels)
	if err != nil {
		return nil, err
	}
	return &ReleaseBatch{
		ID: id, DialectSite: strings.TrimSpace(site), PhoneticSystem: strings.TrimSpace(system),
		AllowedLabels: normalized, MinimumSegmentMillis: minimum, RequireDual: requireDual,
		State: StateDraft, Version: 1, CreatedAt: now.UTC(), Segments: map[string]RecordingSegment{},
		Assignments: map[string][]string{}, Submissions: map[string][]AnnotationSubmission{},
		Decisions: map[string]AdjudicationDecision{}, VerificationRuns: []VerificationRun{}, Repairs: []RepairRequest{},
	}, nil
}

func NormalizeLabels(labels []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			return nil, Invalid("allowedLabels", "标签不能为空")
		}
		if !seen[label] {
			seen[label] = true
			out = append(out, label)
		}
	}
	if len(out) == 0 {
		return nil, Invalid("allowedLabels", "至少配置一个允许标签")
	}
	sort.Strings(out)
	return out, nil
}

func (b *ReleaseBatch) EnsureWritable() error {
	if b.State == StateSealed || b.Credential != nil {
		return ErrSealed
	}
	return nil
}

func (b *ReleaseBatch) CheckVersion(expected uint64) error {
	if expected != b.Version {
		return ErrVersionConflict
	}
	return nil
}

func (b *ReleaseBatch) HasLabel(label string) bool {
	i := sort.SearchStrings(b.AllowedLabels, label)
	return i < len(b.AllowedLabels) && b.AllowedLabels[i] == label
}

func (b *ReleaseBatch) UpdateSpecification(site, system string, labels []string, minimum int64, requireDual bool) error {
	if err := b.EnsureWritable(); err != nil {
		return err
	}
	if b.State != StateDraft {
		return ErrInvalidState
	}
	if strings.TrimSpace(site) == "" || strings.TrimSpace(system) == "" {
		return Invalid("specification", "方言点和音标体系不能为空")
	}
	if minimum <= 0 {
		return Invalid("minimumSegmentMillis", "最短片段时长必须大于零")
	}
	normalized, err := NormalizeLabels(labels)
	if err != nil {
		return err
	}
	for _, segment := range b.Segments {
		if segment.EndMillis-segment.StartMillis < minimum {
			return Invalid("minimumSegmentMillis", "现有片段短于新的时长下限")
		}
	}
	b.DialectSite = strings.TrimSpace(site)
	b.PhoneticSystem = strings.TrimSpace(system)
	b.AllowedLabels = normalized
	b.MinimumSegmentMillis = minimum
	b.RequireDual = requireDual
	b.Version++
	return nil
}

func (b *ReleaseBatch) Clone() *ReleaseBatch {
	copyBatch := *b
	copyBatch.AllowedLabels = append([]string(nil), b.AllowedLabels...)
	copyBatch.Segments = make(map[string]RecordingSegment, len(b.Segments))
	for key, value := range b.Segments {
		copyBatch.Segments[key] = value
	}
	copyBatch.Assignments = make(map[string][]string, len(b.Assignments))
	for key, value := range b.Assignments {
		copyBatch.Assignments[key] = append([]string(nil), value...)
	}
	copyBatch.Submissions = make(map[string][]AnnotationSubmission, len(b.Submissions))
	for key, list := range b.Submissions {
		cloned := make([]AnnotationSubmission, len(list))
		for i, value := range list {
			cloned[i] = value
			cloned[i].Intervals = append([]PhonemeInterval(nil), value.Intervals...)
		}
		copyBatch.Submissions[key] = cloned
	}
	copyBatch.Decisions = make(map[string]AdjudicationDecision, len(b.Decisions))
	for key, value := range b.Decisions {
		value.CandidateLabels = append([]string(nil), value.CandidateLabels...)
		copyBatch.Decisions[key] = value
	}
	copyBatch.VerificationRuns = make([]VerificationRun, len(b.VerificationRuns))
	for i, run := range b.VerificationRuns {
		copyBatch.VerificationRuns[i] = run
		copyBatch.VerificationRuns[i].Scope = append([]string(nil), run.Scope...)
		copyBatch.VerificationRuns[i].Findings = append([]Finding(nil), run.Findings...)
		copyBatch.VerificationRuns[i].Difference = append([]string(nil), run.Difference...)
	}
	copyBatch.Repairs = make([]RepairRequest, len(b.Repairs))
	for i, repair := range b.Repairs {
		copyBatch.Repairs[i] = repair
		copyBatch.Repairs[i].Difference = append([]FindingDifference(nil), repair.Difference...)
	}
	if b.Credential != nil {
		value := *b.Credential
		value.Manifest = append([]ManifestInterval(nil), b.Credential.Manifest...)
		copyBatch.Credential = &value
	}
	return &copyBatch
}
