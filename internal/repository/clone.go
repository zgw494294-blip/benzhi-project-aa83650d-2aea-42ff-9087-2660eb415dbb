package repository

import (
	"fmt"
	"time"

	"acousticverdictworkbench/internal/domain"
)

// cloneBatch returns a fully independent deep copy of the batch so that domain
// mutations applied to the clone can never alias the stored aggregate. This is
// essential for the optimistic-concurrency contract: a service loads a batch,
// mutates the clone, then asks the store to commit it; if the store rejects the
// commit (for example because expectedVersion is stale), the stored batch must
// remain byte-for-byte unchanged. A shallow struct copy would share the backing
// arrays of Submissions/Disputes/Clips/ReannotationTasks, letting in-place
// domain edits corrupt the stored aggregate before the version check runs.
func cloneBatch(batch *domain.ReviewBatch) (*domain.ReviewBatch, error) {
	if batch == nil {
		return nil, fmt.Errorf("不能复制空批次")
	}
	copy := *batch

	copy.AllowedSpeciesCodes = copyStringSlice(batch.AllowedSpeciesCodes)
	copy.Clips = copyAudioClips(batch.Clips)
	copy.Submissions = copySubmissions(batch.Submissions)
	copy.Disputes = copyDisputes(batch.Disputes)
	copy.ReannotationTasks = copyReannotationTasks(batch.ReannotationTasks)
	copy.AdjudicationTrail = copyAdjudicationTrail(batch.AdjudicationTrail)
	copy.LastQuality = copyQualityReport(batch.LastQuality)
	copy.Manifest = copyManifest(batch.Manifest)
	copy.Events = copyEvents(batch.Events)
	copy.FrozenAt = copyTime(batch.FrozenAt)
	copy.ReleasedAt = copyTime(batch.ReleasedAt)
	return &copy, nil
}

func copyStringSlice(values []string) []string {
	if values == nil {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func copyTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	value := *t
	return &value
}

func copyAudioClips(clips []domain.AudioClip) []domain.AudioClip {
	if clips == nil {
		return nil
	}
	out := make([]domain.AudioClip, len(clips))
	copy(out, clips)
	return out
}

func copySubmissions(submissions []domain.AnnotationSubmission) []domain.AnnotationSubmission {
	if submissions == nil {
		return nil
	}
	out := make([]domain.AnnotationSubmission, len(submissions))
	for i, s := range submissions {
		s.Events = append([]domain.CandidateEvent(nil), s.Events...)
		s.SubmittedAt = copyTime(s.SubmittedAt)
		out[i] = s
	}
	return out
}

func copyResolution(resolution *domain.Resolution) *domain.Resolution {
	if resolution == nil {
		return nil
	}
	value := *resolution
	if resolution.NormalizedEvent != nil {
		event := *resolution.NormalizedEvent
		value.NormalizedEvent = &event
	}
	return &value
}

func copyDisputes(disputes []domain.DisputeCase) []domain.DisputeCase {
	if disputes == nil {
		return nil
	}
	out := make([]domain.DisputeCase, len(disputes))
	for i, d := range disputes {
		d.Resolution = copyResolution(d.Resolution)
		d.ResolvedAt = copyTime(d.ResolvedAt)
		out[i] = d
	}
	return out
}

func copyReannotationTasks(tasks []domain.ReannotationTask) []domain.ReannotationTask {
	if tasks == nil {
		return nil
	}
	out := make([]domain.ReannotationTask, len(tasks))
	for i, task := range tasks {
		task.CompletedAt = copyTime(task.CompletedAt)
		out[i] = task
	}
	return out
}

func copyAdjudicationTrail(trail []domain.AdjudicationRecord) []domain.AdjudicationRecord {
	if trail == nil {
		return nil
	}
	out := make([]domain.AdjudicationRecord, len(trail))
	copy(out, trail)
	return out
}

func copyQualityReport(report *domain.QualityReport) *domain.QualityReport {
	if report == nil {
		return nil
	}
	value := *report
	value.Issues = append([]domain.QualityIssue(nil), report.Issues...)
	return &value
}

func copyManifest(manifest *domain.ReleaseManifest) *domain.ReleaseManifest {
	if manifest == nil {
		return nil
	}
	value := *manifest
	value.NormalizedEvents = append([]domain.NormalizedEvent(nil), manifest.NormalizedEvents...)
	value.SourceClips = append([]domain.ClipSummary(nil), manifest.SourceClips...)
	value.AdjudicationTrail = append([]domain.AdjudicationRecord(nil), manifest.AdjudicationTrail...)
	return &value
}

func copyEvents(events []domain.DomainEvent) []domain.DomainEvent {
	if events == nil {
		return nil
	}
	out := make([]domain.DomainEvent, len(events))
	for i, event := range events {
		if event.Details != nil {
			event.Details = copyDetails(event.Details)
		}
		out[i] = event
	}
	return out
}

func copyDetails(details map[string]any) map[string]any {
	out := make(map[string]any, len(details))
	for k, v := range details {
		out[k] = v
	}
	return out
}
