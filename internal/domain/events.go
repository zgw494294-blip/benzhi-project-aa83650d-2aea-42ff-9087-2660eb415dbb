package domain

import "time"

func (b *ReviewBatch) record(kind string, now time.Time, details map[string]any) {
	b.Version++
	b.Events = append(b.Events, DomainEvent{Type: kind, BatchID: b.ID, Version: b.Version, OccurredAt: now.UTC(), Details: details})
}

func (b *ReviewBatch) DrainEvents() []DomainEvent {
	result := append([]DomainEvent(nil), b.Events...)
	b.Events = nil
	return result
}

func (b *ReviewBatch) TouchQuality(report QualityReport, now time.Time) {
	b.LastQuality = &report
	b.record("quality.checked", now, map[string]any{"passed": report.Passed, "issueCount": len(report.Issues)})
}
