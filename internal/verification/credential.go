package verification

import (
	"time"

	"phonemereleasedesk/internal/domain"
)

func IssueCredential(batch *domain.ReleaseBatch, id, reviewer string, now time.Time) (domain.ReleaseCredential, error) {
	manifest, err := BuildManifest(batch)
	if err != nil {
		return domain.ReleaseCredential{}, err
	}
	digest, err := Digest(batch, manifest)
	if err != nil {
		return domain.ReleaseCredential{}, err
	}
	return domain.ReleaseCredential{ID: id, BatchID: batch.ID, ManifestVersion: "phoneme-manifest/1.0", ManifestDigest: digest, SegmentCount: len(batch.Segments), IntervalCount: len(manifest), ReviewerID: reviewer, IssuedAt: now.UTC(), Manifest: manifest}, nil
}
