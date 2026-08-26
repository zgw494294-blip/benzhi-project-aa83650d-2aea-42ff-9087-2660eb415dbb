package domain

import "time"

func (b *ReviewBatch) Seal(manifest ReleaseManifest, now time.Time) error {
	if b.Status == BatchReleased || b.Manifest != nil {
		return ErrAlreadyReleased
	}
	if b.Status != BatchReady {
		return ErrStateConflict
	}
	if b.LastQuality == nil || !b.LastQuality.Passed || b.LastQuality.BatchVersion != b.Version-1 {
		return Invalid("quality", "封存前必须基于当前业务版本通过质量检查")
	}
	if manifest.ID == "" || manifest.ManifestSHA256 == "" || manifest.ReleasedBy == "" {
		return Invalid("manifest", "发布清单凭据不完整")
	}
	manifest.BatchID = b.ID
	b.Manifest = &manifest
	b.Status = BatchReleased
	t := now.UTC()
	b.ReleasedAt = &t
	b.record("batch.released", now, map[string]any{"manifestId": manifest.ID, "sha256": manifest.ManifestSHA256})
	return nil
}

func (b *ReviewBatch) OpenDisputes() []DisputeCase {
	result := []DisputeCase{}
	for _, d := range b.Disputes {
		if !d.Superseded && d.Kind != DisputeAgreement && d.Status == DisputeOpen {
			result = append(result, d)
		}
	}
	return result
}
