package domain

import (
	"regexp"
	"sort"
	"strings"
)

var codePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_-]{1,31}$`)

func NormalizeSpecies(codes []string) ([]string, error) {
	seen := make(map[string]struct{}, len(codes))
	result := make([]string, 0, len(codes))
	for _, raw := range codes {
		code := strings.ToUpper(strings.TrimSpace(raw))
		if !codePattern.MatchString(code) {
			return nil, Invalid("allowedSpeciesCodes", "物种代码必须由大写字母开头，且仅含字母、数字、下划线或连字符")
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	if len(result) == 0 {
		return nil, Invalid("allowedSpeciesCodes", "至少需要一个允许物种代码")
	}
	sort.Strings(result)
	return result, nil
}

func NormalizeCandidate(event CandidateEvent, allowed map[string]struct{}, duration int64) (CandidateEvent, error) {
	event.SpeciesCode = strings.ToUpper(strings.TrimSpace(event.SpeciesCode))
	event.EvidenceNote = strings.TrimSpace(event.EvidenceNote)
	if _, ok := allowed[event.SpeciesCode]; !ok {
		return CandidateEvent{}, Invalid("speciesCode", "候选事件使用了批次范围外的物种代码")
	}
	if event.StartMs < 0 || event.EndMs <= event.StartMs || event.EndMs > duration {
		return CandidateEvent{}, Invalid("timeRange", "候选事件时间区间必须位于片段时长内")
	}
	switch event.Confidence {
	case ConfidenceLow, ConfidenceMedium, ConfidenceHigh:
	default:
		return CandidateEvent{}, Invalid("confidence", "置信等级必须为 low、medium 或 high")
	}
	if event.EvidenceNote == "" {
		return CandidateEvent{}, Invalid("evidenceNote", "候选事件必须填写证据说明")
	}
	if len([]rune(event.EvidenceNote)) > 1000 {
		return CandidateEvent{}, Invalid("evidenceNote", "证据说明不能超过 1000 个字符")
	}
	return event, nil
}

func NormalizeCandidates(events []CandidateEvent, submissionID string, allowed map[string]struct{}, duration int64) ([]CandidateEvent, error) {
	normalized := make([]CandidateEvent, len(events))
	errors := make([]FieldError, 0)
	seenIDs := make(map[string]struct{}, len(events))
	seenEvents := make(map[string]int, len(events))
	for i := range events {
		row := i + 1
		event := events[i]
		event.ID = strings.TrimSpace(event.ID)
		event.SubmissionID = submissionID
		if event.ID == "" || len(event.ID) > 128 {
			errors = append(errors, FieldError{Row: row, Field: "id", Message: "事件标识不能为空且不能超过 128 字符"})
		} else if _, exists := seenIDs[event.ID]; exists {
			errors = append(errors, FieldError{Row: row, Field: "id", Message: "事件标识重复"})
		} else {
			seenIDs[event.ID] = struct{}{}
		}
		candidate, err := NormalizeCandidate(event, allowed, duration)
		if err != nil {
			if rule, ok := err.(*RuleError); ok {
				errors = append(errors, FieldError{Row: row, Field: rule.Field, Message: rule.Message})
			} else {
				errors = append(errors, FieldError{Row: row, Field: "event", Message: err.Error()})
			}
			normalized[i] = event
			continue
		}
		key := candidate.SpeciesCode + "|" + itoaDomain(candidate.StartMs) + "|" + itoaDomain(candidate.EndMs) + "|" + string(candidate.Confidence) + "|" + candidate.EvidenceNote
		if first, exists := seenEvents[key]; exists {
			errors = append(errors, FieldError{Row: row, Field: "event", Message: "候选事件与第 " + itoaDomain(int64(first)) + " 行完全重复"})
		} else {
			seenEvents[key] = row
		}
		normalized[i] = candidate
	}
	if len(errors) > 0 {
		return nil, InvalidFields(errors)
	}
	return normalized, nil
}

func itoaDomain(value int64) string {
	if value == 0 {
		return "0"
	}
	buffer := [24]byte{}
	position := len(buffer)
	for value > 0 {
		position--
		buffer[position] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[position:])
}
