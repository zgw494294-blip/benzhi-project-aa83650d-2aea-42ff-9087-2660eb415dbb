package domain

type BatchStatus string

const (
	BatchDraft        BatchStatus = "draft"
	BatchFrozen       BatchStatus = "frozen"
	BatchAnnotating   BatchStatus = "annotating"
	BatchAdjudicating BatchStatus = "adjudicating"
	BatchReady        BatchStatus = "ready"
	BatchReleased     BatchStatus = "released"
)

type SubmissionStatus string

const (
	SubmissionDraft     SubmissionStatus = "draft"
	SubmissionSubmitted SubmissionStatus = "submitted"
	SubmissionReopened  SubmissionStatus = "reopened"
)

type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

type DisputeKind string

const (
	DisputeAgreement DisputeKind = "agreement"
	DisputeLeftOnly  DisputeKind = "left_only"
	DisputeRightOnly DisputeKind = "right_only"
	DisputeConflict  DisputeKind = "conflict"
)

type DisputeStatus string

const (
	DisputeOpen     DisputeStatus = "open"
	DisputeResolved DisputeStatus = "resolved"
	DisputeReturned DisputeStatus = "returned"
)

type ResolutionKind string

const (
	ResolutionAcceptLeft  ResolutionKind = "accept_left"
	ResolutionAcceptRight ResolutionKind = "accept_right"
	ResolutionMerge       ResolutionKind = "merge"
	ResolutionNoEvent     ResolutionKind = "no_event"
	ResolutionReturn      ResolutionKind = "return"
)

type ReannotationStatus string

const (
	ReannotationPending    ReannotationStatus = "pending"
	ReannotationInProgress ReannotationStatus = "in_progress"
	ReannotationClosed     ReannotationStatus = "closed"
)
