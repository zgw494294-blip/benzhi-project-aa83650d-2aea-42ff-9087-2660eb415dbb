package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type ItemProblem struct {
	Row         int    `json:"row,omitempty"`
	Field       string `json:"field"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	SegmentID   string `json:"segmentId,omitempty"`
	IntervalKey string `json:"intervalKey,omitempty"`
}

type BatchValidationError struct {
	Message  string        `json:"message"`
	Problems []ItemProblem `json:"problems"`
}

func (e BatchValidationError) Error() string { return e.Message }

type SegmentPreflight struct {
	BatchID       string             `json:"batchId"`
	BatchVersion  uint64             `json:"batchVersion"`
	ContentDigest string             `json:"contentDigest"`
	Segments      []RecordingSegment `json:"segments"`
	Problems      []ItemProblem      `json:"problems"`
	Valid         bool               `json:"valid"`
}

func NormalizeSegment(segment RecordingSegment) RecordingSegment {
	segment.ID = strings.TrimSpace(segment.ID)
	segment.SourceRef = strings.TrimSpace(segment.SourceRef)
	segment.SpeakerCode = strings.TrimSpace(segment.SpeakerCode)
	segment.PromptText = strings.TrimSpace(segment.PromptText)
	return segment
}

func SegmentContentDigest(segments []RecordingSegment) string {
	canonical := make([]RecordingSegment, len(segments))
	for i, segment := range segments {
		segment = NormalizeSegment(segment)
		segment.BatchID, segment.Ordinal = "", 0
		canonical[i] = segment
	}
	data, _ := json.Marshal(canonical)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (b *ReleaseBatch) PreflightSegments(input []RecordingSegment) (SegmentPreflight, error) {
	if err := b.EnsureWritable(); err != nil {
		return SegmentPreflight{}, err
	}
	if b.State != StateDraft {
		return SegmentPreflight{}, fmt.Errorf("%w: 仅草拟状态可批量预检或登记片段，当前状态为 %s", ErrInvalidState, b.State)
	}
	segments := make([]RecordingSegment, len(input))
	problems := make([]ItemProblem, 0)
	if len(input) == 0 {
		problems = append(problems, ItemProblem{Field: "segments", Code: "required", Message: "至少提供一行片段"})
	}
	seenIDs := map[string]int{}
	for i, raw := range input {
		row := i + 1
		segment := NormalizeSegment(raw)
		segments[i] = segment
		missing := []struct{ field, value string }{{"id", segment.ID}, {"sourceRef", segment.SourceRef}, {"speakerCode", segment.SpeakerCode}, {"promptText", segment.PromptText}}
		for _, item := range missing {
			if item.value == "" {
				problems = append(problems, ItemProblem{Row: row, Field: item.field, Code: "required", Message: "必填值不能为空", SegmentID: segment.ID})
			}
		}
		if segment.StartMillis < 0 || segment.EndMillis <= segment.StartMillis {
			problems = append(problems, ItemProblem{Row: row, Field: "interval", Code: "invalid_boundary", Message: "片段时间边界无效", SegmentID: segment.ID})
		} else if segment.EndMillis-segment.StartMillis < b.MinimumSegmentMillis {
			problems = append(problems, ItemProblem{Row: row, Field: "interval", Code: "too_short", Message: "片段短于规范下限", SegmentID: segment.ID})
		}
		if _, exists := b.Segments[segment.ID]; segment.ID != "" && exists {
			problems = append(problems, ItemProblem{Row: row, Field: "id", Code: "duplicate_existing", Message: "片段编号已存在", SegmentID: segment.ID})
		}
		if first, exists := seenIDs[segment.ID]; segment.ID != "" && exists {
			problems = append(problems, ItemProblem{Row: row, Field: "id", Code: "duplicate_row", Message: "片段编号与第 " + itoa(first) + " 行重复", SegmentID: segment.ID})
		} else if segment.ID != "" {
			seenIDs[segment.ID] = row
		}
		for _, current := range b.Segments {
			if overlaps(segment, current) {
				problems = append(problems, ItemProblem{Row: row, Field: "interval", Code: "overlap_existing", Message: "与现有片段 " + current.ID + " 的同来源区间重叠", SegmentID: segment.ID})
			}
		}
	}
	for i := range segments {
		for j := i + 1; j < len(segments); j++ {
			if overlaps(segments[i], segments[j]) {
				problems = append(problems,
					ItemProblem{Row: i + 1, Field: "interval", Code: "overlap_row", Message: "与第 " + itoa(j+1) + " 行的同来源区间重叠", SegmentID: segments[i].ID},
					ItemProblem{Row: j + 1, Field: "interval", Code: "overlap_row", Message: "与第 " + itoa(i+1) + " 行的同来源区间重叠", SegmentID: segments[j].ID})
			}
		}
	}
	return SegmentPreflight{BatchID: b.ID, BatchVersion: b.Version, ContentDigest: SegmentContentDigest(segments), Segments: segments, Problems: problems, Valid: len(problems) == 0}, nil
}

func overlaps(left, right RecordingSegment) bool {
	return left.SourceRef != "" && left.SourceRef == right.SourceRef && left.EndMillis > left.StartMillis && right.EndMillis > right.StartMillis && left.StartMillis < right.EndMillis && right.StartMillis < left.EndMillis
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := [20]byte{}
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}

func (b *ReleaseBatch) AddSegments(input []RecordingSegment) error {
	preflight, err := b.PreflightSegments(input)
	if err != nil {
		return err
	}
	if !preflight.Valid {
		return BatchValidationError{Message: "批量片段确认失败", Problems: preflight.Problems}
	}
	ordinal := 0
	for _, segment := range b.Segments {
		if segment.Ordinal > ordinal {
			ordinal = segment.Ordinal
		}
	}
	for _, segment := range preflight.Segments {
		ordinal++
		segment.BatchID, segment.Ordinal = b.ID, ordinal
		b.Segments[segment.ID] = segment
	}
	b.Version++
	return nil
}

type AssignmentPlanItem struct {
	SegmentID string `json:"segmentId"`
	First     string `json:"firstAnnotatorId"`
	Second    string `json:"secondAnnotatorId"`
}

type AnnotatorLoad struct {
	AnnotatorID string `json:"annotatorId"`
	Existing    int    `json:"existing"`
	Added       int    `json:"added"`
	Total       int    `json:"total"`
}

type AssignmentPreview struct {
	BatchID      string               `json:"batchId"`
	BatchVersion uint64               `json:"batchVersion"`
	Plan         []AssignmentPlanItem `json:"plan"`
	Problems     []ItemProblem        `json:"problems"`
	Loads        []AnnotatorLoad      `json:"loads"`
	Unassigned   []string             `json:"unassignedSegmentIds"`
	Valid        bool                 `json:"valid"`
}

func (b *ReleaseBatch) PreviewAssignments(input []AssignmentPlanItem) (AssignmentPreview, error) {
	if err := b.EnsureWritable(); err != nil {
		return AssignmentPreview{}, err
	}
	if b.State != StateFrozen && b.State != StateAnnotating {
		return AssignmentPreview{}, fmt.Errorf("%w: 仅范围冻结或标注状态可批量分配，当前状态为 %s", ErrInvalidState, b.State)
	}
	plan := make([]AssignmentPlanItem, len(input))
	problems := []ItemProblem{}
	if len(input) == 0 {
		problems = append(problems, ItemProblem{Field: "plan", Code: "required", Message: "至少提供一项分配"})
	}
	seen := map[string]bool{}
	validPlan := map[string]bool{}
	added := map[string]int{}
	existing := map[string]int{}
	for _, pair := range b.Assignments {
		for _, actor := range pair {
			existing[actor]++
		}
	}
	for i, raw := range input {
		item := AssignmentPlanItem{SegmentID: strings.TrimSpace(raw.SegmentID), First: strings.TrimSpace(raw.First), Second: strings.TrimSpace(raw.Second)}
		plan[i] = item
		row, valid := i+1, true
		if item.SegmentID == "" {
			problems = append(problems, ItemProblem{Row: row, Field: "segmentId", Code: "required", Message: "片段编号不能为空"})
			valid = false
		}
		if _, ok := b.Segments[item.SegmentID]; !ok {
			problems = append(problems, ItemProblem{Row: row, Field: "segmentId", Code: "segment_not_found", Message: "片段不属于当前冻结范围", SegmentID: item.SegmentID})
			valid = false
		}
		if seen[item.SegmentID] {
			problems = append(problems, ItemProblem{Row: row, Field: "segmentId", Code: "duplicate_row", Message: "同一片段在计划中重复列出", SegmentID: item.SegmentID})
			valid = false
		}
		seen[item.SegmentID] = true
		if item.First == "" || item.Second == "" {
			problems = append(problems, ItemProblem{Row: row, Field: "annotators", Code: "required", Message: "两名标注员均不能为空", SegmentID: item.SegmentID})
			valid = false
		} else if item.First == item.Second {
			problems = append(problems, ItemProblem{Row: row, Field: "annotators", Code: "same_annotator", Message: "必须分配两名不同标注员", SegmentID: item.SegmentID})
			valid = false
		}
		if _, assigned := b.Assignments[item.SegmentID]; assigned {
			problems = append(problems, ItemProblem{Row: row, Field: "segmentId", Code: "already_assigned", Message: "片段已有分配，不允许覆盖", SegmentID: item.SegmentID})
			valid = false
		}
		if len(b.Submissions[item.SegmentID]) > 0 {
			problems = append(problems, ItemProblem{Row: row, Field: "segmentId", Code: "draft_exists", Message: "片段已有标注草稿，不得重新分配", SegmentID: item.SegmentID})
			valid = false
		}
		if valid {
			validPlan[item.SegmentID] = true
			added[item.First]++
			added[item.Second]++
		}
	}
	actors := map[string]bool{}
	for actor := range existing {
		actors[actor] = true
	}
	for actor := range added {
		actors[actor] = true
	}
	loads := make([]AnnotatorLoad, 0, len(actors))
	for actor := range actors {
		loads = append(loads, AnnotatorLoad{AnnotatorID: actor, Existing: existing[actor], Added: added[actor], Total: existing[actor] + added[actor]})
	}
	sort.Slice(loads, func(i, j int) bool { return loads[i].AnnotatorID < loads[j].AnnotatorID })
	unassigned := []string{}
	for _, segment := range b.OrderedSegments() {
		if _, ok := b.Assignments[segment.ID]; !ok && !validPlan[segment.ID] {
			unassigned = append(unassigned, segment.ID)
		}
	}
	return AssignmentPreview{BatchID: b.ID, BatchVersion: b.Version, Plan: plan, Problems: problems, Loads: loads, Unassigned: unassigned, Valid: len(problems) == 0}, nil
}

func (b *ReleaseBatch) AssignMany(input []AssignmentPlanItem) error {
	preview, err := b.PreviewAssignments(input)
	if err != nil {
		return err
	}
	if !preview.Valid {
		return BatchValidationError{Message: "批量分配确认失败", Problems: preview.Problems}
	}
	for _, item := range preview.Plan {
		b.Assignments[item.SegmentID] = []string{item.First, item.Second}
	}
	b.State = StateAnnotating
	b.Version++
	return nil
}

type BulkDecision struct {
	SegmentID     string `json:"segmentId"`
	IntervalKey   string `json:"intervalKey"`
	ResolvedLabel string `json:"resolvedLabel"`
	Reason        string `json:"reason"`
	AdjudicatorID string `json:"adjudicatorId"`
}

func (b *ReleaseBatch) DecideMany(input []BulkDecision, actor string, now time.Time) error {
	if err := b.EnsureWritable(); err != nil {
		return err
	}
	if b.State != StateAdjudicating && b.State != StateRepair {
		return fmt.Errorf("%w: 仅待裁定或返修状态可批量裁定，当前状态为 %s", ErrInvalidState, b.State)
	}
	problems := []ItemProblem{}
	if len(input) == 0 {
		problems = append(problems, ItemProblem{Field: "decisions", Code: "required", Message: "至少提供一项裁定"})
	}
	seen := map[string]bool{}
	normalized := make([]BulkDecision, len(input))
	for i, raw := range input {
		item := BulkDecision{SegmentID: strings.TrimSpace(raw.SegmentID), IntervalKey: strings.TrimSpace(raw.IntervalKey), ResolvedLabel: strings.TrimSpace(raw.ResolvedLabel), Reason: strings.TrimSpace(raw.Reason), AdjudicatorID: strings.TrimSpace(raw.AdjudicatorID)}
		normalized[i] = item
		row := i + 1
		key := DecisionKey(item.SegmentID, item.IntervalKey)
		decision, exists := b.Decisions[key]
		if seen[key] {
			problems = append(problems, ItemProblem{Row: row, Field: "intervalKey", Code: "duplicate_row", Message: "裁定项重复列出", SegmentID: item.SegmentID, IntervalKey: item.IntervalKey})
		}
		seen[key] = true
		if !exists {
			problems = append(problems, ItemProblem{Row: row, Field: "intervalKey", Code: "decision_not_found", Message: "裁定项不存在", SegmentID: item.SegmentID, IntervalKey: item.IntervalKey})
			continue
		}
		if decision.DecidedAt != nil && !decision.Unlocked {
			problems = append(problems, ItemProblem{Row: row, Field: "intervalKey", Code: "already_decided", Message: "裁定项已被处理", SegmentID: item.SegmentID, IntervalKey: item.IntervalKey})
		}
		if b.State == StateRepair && !decision.Unlocked {
			problems = append(problems, ItemProblem{Row: row, Field: "intervalKey", Code: "not_unlocked", Message: "返修中的裁定项未解锁", SegmentID: item.SegmentID, IntervalKey: item.IntervalKey})
		}
		if item.AdjudicatorID == "" || item.AdjudicatorID != strings.TrimSpace(actor) {
			problems = append(problems, ItemProblem{Row: row, Field: "adjudicatorId", Code: "actor_mismatch", Message: "裁定员身份必须与当前操作者一致", SegmentID: item.SegmentID, IntervalKey: item.IntervalKey})
		}
		if item.Reason == "" {
			problems = append(problems, ItemProblem{Row: row, Field: "reason", Code: "required", Message: "裁定理由不能为空", SegmentID: item.SegmentID, IntervalKey: item.IntervalKey})
		}
		if !b.HasLabel(item.ResolvedLabel) {
			problems = append(problems, ItemProblem{Row: row, Field: "resolvedLabel", Code: "label_not_allowed", Message: "裁定标签不在白名单中", SegmentID: item.SegmentID, IntervalKey: item.IntervalKey})
		}
	}
	if len(problems) > 0 {
		return BatchValidationError{Message: "批量裁定失败", Problems: problems}
	}
	instant := now.UTC()
	for _, item := range normalized {
		key := DecisionKey(item.SegmentID, item.IntervalKey)
		d := b.Decisions[key]
		d.ResolvedLabel = item.ResolvedLabel
		d.Reason = item.Reason
		d.AdjudicatorID = item.AdjudicatorID
		d.DecidedAt = &instant
		d.Unlocked = false
		b.Decisions[key] = d
		b.resolveRepairs("adjudication", item.SegmentID, "", item.IntervalKey, now)
	}
	if b.State != StateRepair && b.allDecided() {
		b.State = StateCandidate
	}
	b.Version++
	return nil
}
