package domain

import "time"

type BatchState string

const (
	StateDraft        BatchState = "draft"
	StateFrozen       BatchState = "frozen"
	StateAnnotating   BatchState = "annotating"
	StateChecking     BatchState = "checking"
	StateAdjudicating BatchState = "adjudicating"
	StateCandidate    BatchState = "candidate"
	StateRepair       BatchState = "repair"
	StateSealed       BatchState = "sealed"
)

type SubmissionStatus string

const (
	SubmissionDraft     SubmissionStatus = "draft"
	SubmissionSubmitted SubmissionStatus = "submitted"
	SubmissionUnlocked  SubmissionStatus = "unlocked"
)

type PhonemeInterval struct {
	StartMillis int64  `json:"startMillis"`
	EndMillis   int64  `json:"endMillis"`
	Label       string `json:"label"`
}

type RecordingSegment struct {
	ID          string `json:"id"`
	BatchID     string `json:"batchId"`
	SourceRef   string `json:"sourceRef"`
	StartMillis int64  `json:"startMillis"`
	EndMillis   int64  `json:"endMillis"`
	SpeakerCode string `json:"speakerCode"`
	PromptText  string `json:"promptText"`
	Ordinal     int    `json:"ordinal"`
}

type AnnotationSubmission struct {
	ID          string            `json:"id"`
	SegmentID   string            `json:"segmentId"`
	AnnotatorID string            `json:"annotatorId"`
	Intervals   []PhonemeInterval `json:"intervals"`
	Revision    int               `json:"revision"`
	Status      SubmissionStatus  `json:"status"`
	SubmittedAt *time.Time        `json:"submittedAt,omitempty"`
}

type AdjudicationDecision struct {
	ID              string     `json:"id"`
	BatchID         string     `json:"batchId"`
	SegmentID       string     `json:"segmentId"`
	IntervalKey     string     `json:"intervalKey"`
	CandidateLabels []string   `json:"candidateLabels"`
	ResolvedLabel   string     `json:"resolvedLabel,omitempty"`
	Reason          string     `json:"reason,omitempty"`
	AdjudicatorID   string     `json:"adjudicatorId,omitempty"`
	DecidedAt       *time.Time `json:"decidedAt,omitempty"`
	Unlocked        bool       `json:"unlocked,omitempty"`
}

type Finding struct {
	Rule      string `json:"rule"`
	SegmentID string `json:"segmentId,omitempty"`
	ActorID   string `json:"actorId,omitempty"`
	Interval  string `json:"interval,omitempty"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
}

type VerificationRun struct {
	ID             string    `json:"id"`
	BatchID        string    `json:"batchId"`
	BatchVersion   uint64    `json:"batchVersion"`
	RuleSetVersion string    `json:"ruleSetVersion"`
	Scope          []string  `json:"scope,omitempty"`
	Findings       []Finding `json:"findings"`
	AgreementRate  float64   `json:"agreementRate"`
	Passed         bool      `json:"passed"`
	CreatedAt      time.Time `json:"createdAt"`
	PreviousRunID  string    `json:"previousRunId,omitempty"`
	Difference     []string  `json:"difference,omitempty"`
}

type RepairRequest struct {
	ID            string              `json:"id"`
	SegmentID     string              `json:"segmentId"`
	Rule          string              `json:"rule"`
	TargetKind    string              `json:"targetKind"`
	AnnotatorID   string              `json:"annotatorId,omitempty"`
	IntervalKey   string              `json:"intervalKey,omitempty"`
	Reason        string              `json:"reason"`
	ReviewerID    string              `json:"reviewerId"`
	CreatedAt     time.Time           `json:"createdAt"`
	ResolvedAt    *time.Time          `json:"resolvedAt,omitempty"`
	RoundID       string              `json:"roundId"`
	Status        string              `json:"status"`
	PreviousRunID string              `json:"previousRunId,omitempty"`
	RecheckRunID  string              `json:"recheckRunId,omitempty"`
	Difference    []FindingDifference `json:"difference,omitempty"`
}

type FindingDifference struct {
	Status  string  `json:"status"`
	Finding Finding `json:"finding"`
	Note    string  `json:"note,omitempty"`
}

type ManifestInterval struct {
	SegmentID   string `json:"segmentId"`
	StartMillis int64  `json:"startMillis"`
	EndMillis   int64  `json:"endMillis"`
	Label       string `json:"label"`
}

type ReleaseCredential struct {
	ID              string             `json:"id"`
	BatchID         string             `json:"batchId"`
	ManifestVersion string             `json:"manifestVersion"`
	ManifestDigest  string             `json:"manifestDigest"`
	SegmentCount    int                `json:"segmentCount"`
	IntervalCount   int                `json:"intervalCount"`
	ReviewerID      string             `json:"reviewerId"`
	IssuedAt        time.Time          `json:"issuedAt"`
	Manifest        []ManifestInterval `json:"manifest"`
}
