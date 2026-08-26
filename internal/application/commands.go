package application

import "phonemereleasedesk/internal/domain"

type Metadata struct {
	ExpectedVersion uint64 `json:"expectedVersion"`
	IdempotencyKey  string `json:"-"`
	ActorID         string `json:"actorId"`
	Role            string `json:"role"`
}

type CreateBatchCommand struct {
	Metadata
	ID                   string   `json:"id"`
	DialectSite          string   `json:"dialectSite"`
	PhoneticSystem       string   `json:"phoneticSystem"`
	AllowedLabels        []string `json:"allowedLabels"`
	MinimumSegmentMillis int64    `json:"minimumSegmentMillis"`
	RequireDual          bool     `json:"requireDual"`
}

type SegmentCommand struct {
	Metadata
	BatchID     string `json:"-"`
	ID          string `json:"id"`
	SourceRef   string `json:"sourceRef"`
	StartMillis int64  `json:"startMillis"`
	EndMillis   int64  `json:"endMillis"`
	SpeakerCode string `json:"speakerCode"`
	PromptText  string `json:"promptText"`
}

type SegmentBatchCommand struct {
	Metadata
	BatchID       string                    `json:"-"`
	ContentDigest string                    `json:"contentDigest,omitempty"`
	Segments      []domain.RecordingSegment `json:"segments"`
}

type SpecificationCommand struct {
	Metadata
	BatchID              string   `json:"-"`
	DialectSite          string   `json:"dialectSite"`
	PhoneticSystem       string   `json:"phoneticSystem"`
	AllowedLabels        []string `json:"allowedLabels"`
	MinimumSegmentMillis int64    `json:"minimumSegmentMillis"`
	RequireDual          bool     `json:"requireDual"`
}

type RemoveSegmentCommand struct {
	Metadata
	BatchID   string `json:"-"`
	SegmentID string `json:"-"`
}

type BatchCommand struct {
	Metadata
	BatchID string `json:"-"`
}

type AssignmentCommand struct {
	Metadata
	BatchID   string `json:"-"`
	SegmentID string `json:"segmentId"`
	First     string `json:"firstAnnotatorId"`
	Second    string `json:"secondAnnotatorId"`
}

type AssignmentBatchCommand struct {
	Metadata
	BatchID string                      `json:"-"`
	Plan    []domain.AssignmentPlanItem `json:"plan"`
}

type SubmissionCommand struct {
	Metadata
	BatchID     string                   `json:"-"`
	SegmentID   string                   `json:"segmentId"`
	AnnotatorID string                   `json:"annotatorId"`
	Intervals   []domain.PhonemeInterval `json:"intervals"`
	Submit      bool                     `json:"submit"`
}

type DecisionCommand struct {
	Metadata
	BatchID       string `json:"-"`
	SegmentID     string `json:"segmentId"`
	IntervalKey   string `json:"intervalKey"`
	ResolvedLabel string `json:"resolvedLabel"`
	Reason        string `json:"reason"`
	AdjudicatorID string `json:"adjudicatorId"`
}

type DecisionBatchCommand struct {
	Metadata
	BatchID   string                `json:"-"`
	Decisions []domain.BulkDecision `json:"decisions"`
}

type RepairCommand struct {
	Metadata
	BatchID     string `json:"-"`
	SegmentID   string `json:"segmentId"`
	Rule        string `json:"rule"`
	TargetKind  string `json:"targetKind"`
	AnnotatorID string `json:"annotatorId,omitempty"`
	IntervalKey string `json:"intervalKey,omitempty"`
	Reason      string `json:"reason"`
	ReviewerID  string `json:"reviewerId"`
}

type SealCommand struct {
	Metadata
	BatchID    string `json:"-"`
	ReviewerID string `json:"reviewerId"`
}
