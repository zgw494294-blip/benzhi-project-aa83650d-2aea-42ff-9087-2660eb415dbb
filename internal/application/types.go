package application

import (
	"time"

	"acousticverdictworkbench/internal/domain"
)

type Metadata struct {
	ActorID         string `json:"actorId"`
	Role            string `json:"role"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	IdempotencyKey  string `json:"-"`
}

type CreateBatchCommand struct {
	Metadata
	Title          string    `json:"title"`
	SiteCode       string    `json:"siteCode"`
	RecordingStart time.Time `json:"recordingStart"`
	RecordingEnd   time.Time `json:"recordingEnd"`
}

type ConfigureScopeCommand struct {
	Metadata
	BatchID             string    `json:"-"`
	Title               string    `json:"title"`
	SiteCode            string    `json:"siteCode"`
	RecordingStart      time.Time `json:"recordingStart"`
	RecordingEnd        time.Time `json:"recordingEnd"`
	AllowedSpeciesCodes []string  `json:"allowedSpeciesCodes"`
}

type AddClipCommand struct {
	Metadata
	BatchID       string    `json:"-"`
	ID            string    `json:"id"`
	SourceName    string    `json:"sourceName"`
	CapturedAt    time.Time `json:"capturedAt"`
	DurationMs    int64     `json:"durationMs"`
	ContentSHA256 string    `json:"contentSHA256"`
	Sequence      int       `json:"sequence"`
}

type BulkRegisterClipsCommand struct {
	Metadata
	BatchID string          `json:"-"`
	Clips   []BulkClipInput `json:"clips"`
}

type BulkClipInput struct {
	ID            string    `json:"id,omitempty"`
	SourceName    string    `json:"sourceName"`
	CapturedAt    time.Time `json:"capturedAt"`
	DurationMs    int64     `json:"durationMs"`
	ContentSHA256 string    `json:"contentSHA256"`
	Sequence      int       `json:"sequence"`
}

type BulkRegisterClipsResult struct {
	Batch      *domain.ReviewBatch `json:"batch"`
	AddedCount int                 `json:"addedCount"`
}

type BatchCommand struct {
	Metadata
	BatchID string `json:"-"`
}

type DraftCommand struct {
	Metadata
	BatchID        string                  `json:"-"`
	ClipID         string                  `json:"-"`
	SubmissionID   string                  `json:"submissionId"`
	AnnotatorID    string                  `json:"annotatorId"`
	Round          int                     `json:"round"`
	RevisionReason string                  `json:"revisionReason"`
	Events         []domain.CandidateEvent `json:"events"`
}

type SubmitCommand struct {
	Metadata
	BatchID        string `json:"-"`
	ClipID         string `json:"-"`
	AnnotatorID    string `json:"annotatorId"`
	Round          int    `json:"round"`
	RevisionReason string `json:"revisionReason,omitempty"`
	Confirmed      bool   `json:"confirmed"`
}

type ResolveCommand struct {
	Metadata
	BatchID    string            `json:"-"`
	DisputeID  string            `json:"-"`
	ReviewerID string            `json:"reviewerId"`
	Resolution domain.Resolution `json:"resolution"`
}

type ReleaseCommand struct {
	Metadata
	BatchID    string `json:"-"`
	ReleasedBy string `json:"releasedBy"`
}

type BatchView struct {
	Batch                *domain.ReviewBatch           `json:"batch"`
	Submissions          []domain.AnnotationSubmission `json:"submissions"`
	OpenQueue            []domain.DisputeCase          `json:"openQueue"`
	ReannotationTasks    []domain.ReannotationTask     `json:"reannotationTasks"`
	ReannotationProgress ReannotationProgress          `json:"reannotationProgress"`
}

type ReannotationProgress struct {
	Pending    int `json:"pending"`
	InProgress int `json:"inProgress"`
	Closed     int `json:"closed"`
}

type ManifestEventQuery struct {
	ClipID      string
	SpeciesCode string
	StartMs     *int64
	EndMs       *int64
	Page        int
	PageSize    int
}

type ManifestDetails struct {
	ManifestID        string                      `json:"manifestId"`
	BatchID           string                      `json:"batchId"`
	Events            []domain.NormalizedEvent    `json:"events"`
	Total             int                         `json:"total"`
	Page              int                         `json:"page"`
	PageSize          int                         `json:"pageSize"`
	SourceClips       []domain.ClipSummary        `json:"sourceClips"`
	AdjudicationTrail []domain.AdjudicationRecord `json:"adjudicationTrail"`
}

type DigestVerification struct {
	Field      string `json:"field"`
	Expected   string `json:"expected"`
	Actual     string `json:"actual"`
	Consistent bool   `json:"consistent"`
}

type ManifestVerification struct {
	ManifestID string               `json:"manifestId"`
	Consistent bool                 `json:"consistent"`
	Items      []DigestVerification `json:"items"`
}
