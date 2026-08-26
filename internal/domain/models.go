package domain

import "time"

type ReviewBatch struct {
	ID                  string                 `json:"id"`
	Title               string                 `json:"title"`
	SiteCode            string                 `json:"siteCode"`
	RecordingStart      time.Time              `json:"recordingStart"`
	RecordingEnd        time.Time              `json:"recordingEnd"`
	AllowedSpeciesCodes []string               `json:"allowedSpeciesCodes"`
	Status              BatchStatus            `json:"status"`
	Version             uint64                 `json:"version"`
	CreatedAt           time.Time              `json:"createdAt"`
	FrozenAt            *time.Time             `json:"frozenAt,omitempty"`
	ReleasedAt          *time.Time             `json:"releasedAt,omitempty"`
	Clips               []AudioClip            `json:"clips"`
	Submissions         []AnnotationSubmission `json:"submissions"`
	Disputes            []DisputeCase          `json:"disputes"`
	ReannotationTasks   []ReannotationTask     `json:"reannotationTasks,omitempty"`
	AdjudicationTrail   []AdjudicationRecord   `json:"adjudicationTrail"`
	LastQuality         *QualityReport         `json:"lastQuality,omitempty"`
	Manifest            *ReleaseManifest       `json:"manifest,omitempty"`
	Events              []DomainEvent          `json:"-"`
}

type AudioClip struct {
	ID            string    `json:"id"`
	BatchID       string    `json:"batchId"`
	SourceName    string    `json:"sourceName"`
	CapturedAt    time.Time `json:"capturedAt"`
	DurationMs    int64     `json:"durationMs"`
	ContentSHA256 string    `json:"contentSHA256"`
	Sequence      int       `json:"sequence"`
	ReviewState   string    `json:"reviewState"`
}

type AnnotationSubmission struct {
	ID             string           `json:"id"`
	ClipID         string           `json:"clipId"`
	AnnotatorID    string           `json:"annotatorId"`
	Round          int              `json:"round"`
	Status         SubmissionStatus `json:"status"`
	Events         []CandidateEvent `json:"events"`
	SubmittedAt    *time.Time       `json:"submittedAt,omitempty"`
	RevisionReason string           `json:"revisionReason,omitempty"`
}

type CandidateEvent struct {
	ID           string     `json:"id"`
	SubmissionID string     `json:"submissionId"`
	SpeciesCode  string     `json:"speciesCode"`
	StartMs      int64      `json:"startMs"`
	EndMs        int64      `json:"endMs"`
	Confidence   Confidence `json:"confidence"`
	EvidenceNote string     `json:"evidenceNote"`
}

type MatchBasis struct {
	SpeciesEqual bool    `json:"speciesEqual"`
	OverlapMs    int64   `json:"overlapMs"`
	UnionMs      int64   `json:"unionMs"`
	TimeIoU      float64 `json:"timeIoU"`
	Explanation  string  `json:"explanation"`
}

type DisputeCase struct {
	ID           string        `json:"id"`
	ClipID       string        `json:"clipId"`
	Kind         DisputeKind   `json:"kind"`
	LeftEventID  string        `json:"leftEventId,omitempty"`
	RightEventID string        `json:"rightEventId,omitempty"`
	MatchScore   float64       `json:"matchScore"`
	Basis        MatchBasis    `json:"basis"`
	Status       DisputeStatus `json:"status"`
	Resolution   *Resolution   `json:"resolution,omitempty"`
	ReviewerID   string        `json:"reviewerId,omitempty"`
	ResolvedAt   *time.Time    `json:"resolvedAt,omitempty"`
	Superseded   bool          `json:"superseded,omitempty"`
}

type ReannotationTask struct {
	ID              string             `json:"id"`
	DisputeID       string             `json:"disputeId"`
	ClipID          string             `json:"clipId"`
	TargetAnnotator string             `json:"targetAnnotator"`
	Round           int                `json:"round"`
	Reason          string             `json:"reason"`
	OriginalKind    DisputeKind        `json:"originalKind"`
	OriginalLeftID  string             `json:"originalLeftEventId,omitempty"`
	OriginalRightID string             `json:"originalRightEventId,omitempty"`
	OriginalBasis   MatchBasis         `json:"originalBasis"`
	Status          ReannotationStatus `json:"status"`
	CreatedAt       time.Time          `json:"createdAt"`
	CompletedAt     *time.Time         `json:"completedAt,omitempty"`
	RevisionReason  string             `json:"revisionReason,omitempty"`
}

type Resolution struct {
	Kind            ResolutionKind  `json:"kind"`
	NormalizedEvent *CandidateEvent `json:"normalizedEvent,omitempty"`
	ReturnAnnotator string          `json:"returnAnnotator,omitempty"`
	Reason          string          `json:"reason"`
}

type AdjudicationRecord struct {
	ID              string         `json:"id"`
	DisputeID       string         `json:"disputeId"`
	ClipID          string         `json:"clipId"`
	Kind            ResolutionKind `json:"kind"`
	ReviewerID      string         `json:"reviewerId"`
	TargetAnnotator string         `json:"targetAnnotator,omitempty"`
	Reason          string         `json:"reason"`
	At              time.Time      `json:"at"`
}

type QualityIssue struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	BatchID      string `json:"batchId"`
	ClipID       string `json:"clipId,omitempty"`
	SubmissionID string `json:"submissionId,omitempty"`
	EventID      string `json:"eventId,omitempty"`
	DisputeID    string `json:"disputeId,omitempty"`
}

type QualityReport struct {
	BatchID      string         `json:"batchId"`
	BatchVersion uint64         `json:"batchVersion"`
	CheckedAt    time.Time      `json:"checkedAt"`
	ClipCount    int            `json:"clipCount"`
	CoveredClips int            `json:"coveredClips"`
	CoverageRate float64        `json:"coverageRate"`
	Passed       bool           `json:"passed"`
	Issues       []QualityIssue `json:"issues"`
}

type NormalizedEvent struct {
	ClipID       string     `json:"clipId"`
	SpeciesCode  string     `json:"speciesCode"`
	StartMs      int64      `json:"startMs"`
	EndMs        int64      `json:"endMs"`
	Confidence   Confidence `json:"confidence"`
	EvidenceNote string     `json:"evidenceNote"`
	Source       string     `json:"source"`
}

type ClipSummary struct {
	ClipID        string `json:"clipId"`
	SourceName    string `json:"sourceName"`
	DurationMs    int64  `json:"durationMs"`
	ContentSHA256 string `json:"contentSHA256"`
}

type ReleaseManifest struct {
	ID                 string               `json:"id"`
	BatchID            string               `json:"batchId"`
	BatchVersion       uint64               `json:"batchVersion"`
	NormalizedEvents   []NormalizedEvent    `json:"normalizedEvents"`
	SourceClips        []ClipSummary        `json:"sourceClips"`
	AdjudicationTrail  []AdjudicationRecord `json:"adjudicationTrail"`
	ClipDigest         string               `json:"clipDigest"`
	AdjudicationDigest string               `json:"adjudicationDigest"`
	ManifestSHA256     string               `json:"manifestSHA256"`
	ReleasedBy         string               `json:"releasedBy"`
	ReleasedAt         time.Time            `json:"releasedAt"`
}

type DomainEvent struct {
	Type       string         `json:"type"`
	BatchID    string         `json:"batchId"`
	Version    uint64         `json:"version"`
	OccurredAt time.Time      `json:"occurredAt"`
	Details    map[string]any `json:"details,omitempty"`
}
