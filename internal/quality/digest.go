package quality

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"acousticverdictworkbench/internal/domain"
)

type canonicalManifest struct {
	BatchID            string                      `json:"batchId"`
	BatchVersion       uint64                      `json:"batchVersion"`
	Events             []domain.NormalizedEvent    `json:"events"`
	Clips              []domain.ClipSummary        `json:"clips"`
	Adjudication       []domain.AdjudicationRecord `json:"adjudication"`
	ClipDigest         string                      `json:"clipDigest"`
	AdjudicationDigest string                      `json:"adjudicationDigest"`
}

type DigestCheck struct {
	Field      string
	Expected   string
	Actual     string
	Consistent bool
}

func BuildManifest(id, releasedBy string, batch *domain.ReviewBatch, at time.Time) (domain.ReleaseManifest, error) {
	events := NormalizeEvents(batch)
	clips := make([]domain.ClipSummary, len(batch.Clips))
	for i, clip := range batch.Clips {
		clips[i] = domain.ClipSummary{ClipID: clip.ID, SourceName: clip.SourceName, DurationMs: clip.DurationMs, ContentSHA256: clip.ContentSHA256}
	}
	sort.Slice(clips, func(i, j int) bool { return clips[i].ClipID < clips[j].ClipID })
	trail := append([]domain.AdjudicationRecord(nil), batch.AdjudicationTrail...)
	sort.Slice(trail, func(i, j int) bool {
		if trail[i].At.Equal(trail[j].At) {
			return trail[i].ID < trail[j].ID
		}
		return trail[i].At.Before(trail[j].At)
	})
	clipDigest, err := digest(clips)
	if err != nil {
		return domain.ReleaseManifest{}, err
	}
	adjDigest, err := digest(trail)
	if err != nil {
		return domain.ReleaseManifest{}, err
	}
	payload := canonicalManifest{BatchID: batch.ID, BatchVersion: batch.Version, Events: events, Clips: clips, Adjudication: trail, ClipDigest: clipDigest, AdjudicationDigest: adjDigest}
	manifestDigest, err := digest(payload)
	if err != nil {
		return domain.ReleaseManifest{}, err
	}
	return domain.ReleaseManifest{ID: id, BatchID: batch.ID, BatchVersion: batch.Version, NormalizedEvents: events, SourceClips: clips, AdjudicationTrail: trail, ClipDigest: clipDigest, AdjudicationDigest: adjDigest, ManifestSHA256: manifestDigest, ReleasedBy: releasedBy, ReleasedAt: at.UTC()}, nil
}

func VerifyManifest(manifest domain.ReleaseManifest) ([]DigestCheck, error) {
	var events []domain.NormalizedEvent
	if manifest.NormalizedEvents != nil {
		events = make([]domain.NormalizedEvent, len(manifest.NormalizedEvents))
		copy(events, manifest.NormalizedEvents)
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].ClipID != events[j].ClipID {
			return events[i].ClipID < events[j].ClipID
		}
		if events[i].StartMs != events[j].StartMs {
			return events[i].StartMs < events[j].StartMs
		}
		if events[i].EndMs != events[j].EndMs {
			return events[i].EndMs < events[j].EndMs
		}
		if events[i].SpeciesCode != events[j].SpeciesCode {
			return events[i].SpeciesCode < events[j].SpeciesCode
		}
		if events[i].Confidence != events[j].Confidence {
			return events[i].Confidence < events[j].Confidence
		}
		if events[i].Source != events[j].Source {
			return events[i].Source < events[j].Source
		}
		return events[i].EvidenceNote < events[j].EvidenceNote
	})
	var clips []domain.ClipSummary
	if manifest.SourceClips != nil {
		clips = make([]domain.ClipSummary, len(manifest.SourceClips))
		copy(clips, manifest.SourceClips)
	}
	sort.Slice(clips, func(i, j int) bool { return clips[i].ClipID < clips[j].ClipID })
	var trail []domain.AdjudicationRecord
	if manifest.AdjudicationTrail != nil {
		trail = make([]domain.AdjudicationRecord, len(manifest.AdjudicationTrail))
		copy(trail, manifest.AdjudicationTrail)
	}
	sort.Slice(trail, func(i, j int) bool {
		if trail[i].At.Equal(trail[j].At) {
			return trail[i].ID < trail[j].ID
		}
		return trail[i].At.Before(trail[j].At)
	})
	clipDigest, err := digest(clips)
	if err != nil {
		return nil, err
	}
	adjudicationDigest, err := digest(trail)
	if err != nil {
		return nil, err
	}
	payload := canonicalManifest{BatchID: manifest.BatchID, BatchVersion: manifest.BatchVersion, Events: events, Clips: clips, Adjudication: trail, ClipDigest: clipDigest, AdjudicationDigest: adjudicationDigest}
	manifestDigest, err := digest(payload)
	if err != nil {
		return nil, err
	}
	return []DigestCheck{
		{Field: "clipDigest", Expected: manifest.ClipDigest, Actual: clipDigest, Consistent: manifest.ClipDigest == clipDigest},
		{Field: "adjudicationDigest", Expected: manifest.AdjudicationDigest, Actual: adjudicationDigest, Consistent: manifest.AdjudicationDigest == adjudicationDigest},
		{Field: "manifestSHA256", Expected: manifest.ManifestSHA256, Actual: manifestDigest, Consistent: manifest.ManifestSHA256 == manifestDigest},
	}, nil
}

func digest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
