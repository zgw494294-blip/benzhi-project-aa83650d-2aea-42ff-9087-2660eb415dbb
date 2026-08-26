package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"phonemereleasedesk/internal/domain"
)

type canonicalManifest struct {
	BatchID        string                    `json:"batchId"`
	DialectSite    string                    `json:"dialectSite"`
	PhoneticSystem string                    `json:"phoneticSystem"`
	RuleSetVersion string                    `json:"ruleSetVersion"`
	Intervals      []domain.ManifestInterval `json:"intervals"`
}

func BuildManifest(batch *domain.ReleaseBatch) ([]domain.ManifestInterval, error) {
	if !batch.CandidateReady() {
		return nil, domain.ErrInvalidState
	}
	return buildManifestFromContent(batch)
}

func buildManifestFromContent(batch *domain.ReleaseBatch) ([]domain.ManifestInterval, error) {
	manifest := []domain.ManifestInterval{}
	for _, segment := range batch.OrderedSegments() {
		submissions := batch.Submissions[segment.ID]
		if len(submissions) != 2 {
			return nil, domain.Invalid("submissions", "无法从不完整标注生成清单")
		}
		left, right := intervalMap(submissions[0].Intervals), intervalMap(submissions[1].Intervals)
		for _, key := range unionKeys(left, right) {
			start, end, err := parseKey(key)
			if err != nil {
				return nil, err
			}
			label := left[key]
			if left[key] != right[key] {
				decision, found := batch.Decisions[domain.DecisionKey(segment.ID, key)]
				if !found || decision.ResolvedLabel == "" {
					return nil, domain.Invalid("decisions", "存在未裁定冲突")
				}
				label = decision.ResolvedLabel
			}
			manifest = append(manifest, domain.ManifestInterval{SegmentID: segment.ID, StartMillis: start, EndMillis: end, Label: label})
		}
	}
	sort.Slice(manifest, func(i, j int) bool {
		if manifest[i].SegmentID != manifest[j].SegmentID {
			return manifest[i].SegmentID < manifest[j].SegmentID
		}
		if manifest[i].StartMillis != manifest[j].StartMillis {
			return manifest[i].StartMillis < manifest[j].StartMillis
		}
		return manifest[i].EndMillis < manifest[j].EndMillis
	})
	return manifest, nil
}

func Digest(batch *domain.ReleaseBatch, manifest []domain.ManifestInterval) (string, error) {
	value := canonicalManifest{BatchID: batch.ID, DialectSite: batch.DialectSite, PhoneticSystem: batch.PhoneticSystem, RuleSetVersion: RuleSetVersion, Intervals: manifest}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func VerifyCredential(batch *domain.ReleaseBatch, credential domain.ReleaseCredential) (bool, string, error) {
	manifest, err := buildManifestFromContent(batch)
	if err != nil {
		return false, "", err
	}
	digest, err := Digest(batch, manifest)
	if err != nil {
		return false, "", err
	}
	return digest == credential.ManifestDigest && len(manifest) == credential.IntervalCount && len(batch.Segments) == credential.SegmentCount, digest, nil
}

type DimensionCheck struct {
	Passed     bool `json:"passed"`
	Stored     any  `json:"stored"`
	Recomputed any  `json:"recomputed"`
}

type CredentialChecks struct {
	Valid         bool           `json:"valid"`
	Digest        DimensionCheck `json:"digest"`
	SegmentCount  DimensionCheck `json:"segmentCount"`
	IntervalCount DimensionCheck `json:"intervalCount"`
}

func VerifyCredentialDimensions(batch *domain.ReleaseBatch, credential domain.ReleaseCredential) (CredentialChecks, error) {
	manifest, err := buildManifestFromContent(batch)
	if err != nil {
		return CredentialChecks{}, err
	}
	digest, err := Digest(batch, manifest)
	if err != nil {
		return CredentialChecks{}, err
	}
	digestCheck := DimensionCheck{Passed: digest == credential.ManifestDigest, Stored: credential.ManifestDigest, Recomputed: digest}
	segmentCheck := DimensionCheck{Passed: len(batch.Segments) == credential.SegmentCount, Stored: credential.SegmentCount, Recomputed: len(batch.Segments)}
	intervalCheck := DimensionCheck{Passed: len(manifest) == credential.IntervalCount, Stored: credential.IntervalCount, Recomputed: len(manifest)}
	return CredentialChecks{Valid: digestCheck.Passed && segmentCheck.Passed && intervalCheck.Passed, Digest: digestCheck, SegmentCount: segmentCheck, IntervalCount: intervalCheck}, nil
}

func BuildManifestForCredential(batch *domain.ReleaseBatch) ([]domain.ManifestInterval, error) {
	if batch.Credential != nil {
		return append([]domain.ManifestInterval(nil), batch.Credential.Manifest...), nil
	}
	return BuildManifest(batch)
}

func parseKey(key string) (int64, int64, error) {
	var start, end int64
	if _, err := fmt.Sscanf(key, "%d-%d", &start, &end); err != nil {
		return 0, 0, domain.Invalid("intervalKey", "区间键格式无效")
	}
	return start, end, nil
}
