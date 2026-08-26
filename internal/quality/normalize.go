package quality

import (
	"sort"

	"acousticverdictworkbench/internal/domain"
)

func NormalizeEvents(batch *domain.ReviewBatch) []domain.NormalizedEvent {
	byID := map[string]domain.CandidateEvent{}
	for _, sub := range batch.Submissions {
		for _, event := range sub.Events {
			byID[event.ID] = event
		}
	}
	result := []domain.NormalizedEvent{}
	seen := map[string]struct{}{}
	appendEvent := func(clipID string, event domain.CandidateEvent, source string) {
		key := clipID + "|" + event.SpeciesCode + "|" + itoa(event.StartMs) + "|" + itoa(event.EndMs)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		result = append(result, domain.NormalizedEvent{ClipID: clipID, SpeciesCode: event.SpeciesCode, StartMs: event.StartMs, EndMs: event.EndMs, Confidence: event.Confidence, EvidenceNote: event.EvidenceNote, Source: source})
	}
	for _, c := range batch.Disputes {
		if c.Superseded {
			continue
		}
		if c.Kind == domain.DisputeAgreement {
			if event, ok := byID[c.LeftEventID]; ok {
				appendEvent(c.ClipID, event, "agreement")
			}
			continue
		}
		if c.Resolution != nil && c.Resolution.NormalizedEvent != nil {
			appendEvent(c.ClipID, *c.Resolution.NormalizedEvent, string(c.Resolution.Kind))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ClipID != result[j].ClipID {
			return result[i].ClipID < result[j].ClipID
		}
		if result[i].StartMs != result[j].StartMs {
			return result[i].StartMs < result[j].StartMs
		}
		if result[i].EndMs != result[j].EndMs {
			return result[i].EndMs < result[j].EndMs
		}
		return result[i].SpeciesCode < result[j].SpeciesCode
	})
	return result
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	negative := v < 0
	if negative {
		v = -v
	}
	buf := [24]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
