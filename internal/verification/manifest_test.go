package verification

import (
	"testing"
	"time"

	"phonemereleasedesk/internal/domain"
)

func candidateBatch(t *testing.T) *domain.ReleaseBatch {
	t.Helper()
	now := time.Unix(100, 0)
	b, err := domain.NewReleaseBatch("b", "点", "IPA", []string{"a", "i"}, 100, true, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.AddSegment(domain.RecordingSegment{ID: "s", SourceRef: "r", StartMillis: 0, EndMillis: 1000, SpeakerCode: "p", PromptText: "字"}); err != nil {
		t.Fatal(err)
	}
	if err := b.Freeze(); err != nil {
		t.Fatal(err)
	}
	if err := b.Assign("s", "x", "y"); err != nil {
		t.Fatal(err)
	}
	intervals := []domain.PhonemeInterval{{StartMillis: 0, EndMillis: 1000, Label: "a"}}
	if err := b.SaveSubmission("s", "x", intervals, true, now); err != nil {
		t.Fatal(err)
	}
	if err := b.SaveSubmission("s", "y", intervals, true, now); err != nil {
		t.Fatal(err)
	}
	if err := b.BeginChecking(); err != nil {
		t.Fatal(err)
	}
	result := Run(b, "run", nil, now)
	if err := b.RecordVerification(result.Run); err != nil {
		t.Fatal(err)
	}
	if err := b.InstallDecisions(result.Conflicts); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDigestIsStable(t *testing.T) {
	b := candidateBatch(t)
	manifest, err := BuildManifest(b)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Digest(b, manifest)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Digest(b, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) != 64 {
		t.Fatalf("摘要不稳定：%q / %q", first, second)
	}
}

func TestIssueAndVerifyCredential(t *testing.T) {
	b := candidateBatch(t)
	credential, err := IssueCredential(b, "c", "reviewer", time.Unix(200, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Seal(credential, "reviewer", time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}
	valid, digest, err := VerifyCredential(b, credential)
	if err != nil {
		t.Fatal(err)
	}
	if !valid || digest != credential.ManifestDigest {
		t.Fatal("凭据核验失败")
	}
}
