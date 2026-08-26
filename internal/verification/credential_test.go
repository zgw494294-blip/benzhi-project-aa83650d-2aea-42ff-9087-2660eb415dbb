package verification

import (
	"testing"
	"time"

	"phonemereleasedesk/internal/domain"
)

func TestCredentialDimensionChecksRemainIndependent(t *testing.T) {
	batch, err := domain.NewReleaseBatch("b", "点", "IPA", []string{"a"}, 100, true, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	segment := domain.RecordingSegment{ID: "s", SourceRef: "R", StartMillis: 0, EndMillis: 100, SpeakerCode: "S", PromptText: "文本"}
	if err := batch.AddSegment(segment); err != nil {
		t.Fatal(err)
	}
	batch.State = domain.StateCandidate
	intervals := []domain.PhonemeInterval{{StartMillis: 0, EndMillis: 100, Label: "a"}}
	batch.Submissions["s"] = []domain.AnnotationSubmission{{AnnotatorID: "x", Status: domain.SubmissionSubmitted, Intervals: intervals}, {AnnotatorID: "y", Status: domain.SubmissionSubmitted, Intervals: intervals}}
	credential, err := IssueCredential(batch, "c", "reviewer", time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	credential.SegmentCount++
	checks, err := VerifyCredentialDimensions(batch, credential)
	if err != nil {
		t.Fatal(err)
	}
	if checks.Valid || !checks.Digest.Passed || checks.SegmentCount.Passed || !checks.IntervalCount.Passed {
		t.Fatalf("分项核验未保持独立：%+v", checks)
	}
}
