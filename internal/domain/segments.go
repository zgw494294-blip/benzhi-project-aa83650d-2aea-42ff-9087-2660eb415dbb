package domain

import (
	"sort"
	"strings"
)

func (b *ReleaseBatch) AddSegment(segment RecordingSegment) error {
	if err := b.EnsureWritable(); err != nil {
		return err
	}
	if b.State != StateDraft {
		return ErrInvalidState
	}
	if strings.TrimSpace(segment.ID) == "" || strings.TrimSpace(segment.SourceRef) == "" || strings.TrimSpace(segment.SpeakerCode) == "" || strings.TrimSpace(segment.PromptText) == "" {
		return Invalid("segment", "片段 ID、来源编号、说话人代号和目标文本均为必填")
	}
	if segment.StartMillis < 0 || segment.EndMillis <= segment.StartMillis {
		return Invalid("interval", "片段时间边界无效")
	}
	if segment.EndMillis-segment.StartMillis < b.MinimumSegmentMillis {
		return Invalid("interval", "片段短于规范下限")
	}
	if _, exists := b.Segments[segment.ID]; exists {
		return Invalid("id", "片段 ID 已存在")
	}
	for _, current := range b.Segments {
		if current.SourceRef == segment.SourceRef && segment.StartMillis < current.EndMillis && current.StartMillis < segment.EndMillis {
			return Invalid("interval", "同一来源的片段区间重叠")
		}
	}
	segment.BatchID = b.ID
	segment.Ordinal = len(b.Segments) + 1
	b.Segments[segment.ID] = segment
	b.Version++
	return nil
}

func (b *ReleaseBatch) RemoveSegment(id string) error {
	if err := b.EnsureWritable(); err != nil {
		return err
	}
	if b.State != StateDraft {
		return ErrInvalidState
	}
	if _, exists := b.Segments[id]; !exists {
		return ErrNotFound
	}
	delete(b.Segments, id)
	b.Version++
	return nil
}

func (b *ReleaseBatch) OrderedSegments() []RecordingSegment {
	result := make([]RecordingSegment, 0, len(b.Segments))
	for _, item := range b.Segments {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Ordinal < result[j].Ordinal })
	return result
}

func (b *ReleaseBatch) Freeze() error {
	if err := b.EnsureWritable(); err != nil {
		return err
	}
	if b.State != StateDraft {
		return ErrInvalidState
	}
	if len(b.Segments) == 0 {
		return Invalid("segments", "冻结前至少登记一个片段")
	}
	b.State = StateFrozen
	b.Version++
	return nil
}

func (b *ReleaseBatch) Assign(segmentID, first, second string) error {
	if err := b.EnsureWritable(); err != nil {
		return err
	}
	if b.State != StateFrozen && b.State != StateAnnotating {
		return ErrInvalidState
	}
	if _, exists := b.Segments[segmentID]; !exists {
		return ErrNotFound
	}
	first, second = strings.TrimSpace(first), strings.TrimSpace(second)
	if first == "" || second == "" || first == second {
		return Invalid("annotators", "必须分配两名不同标注员")
	}
	if _, assigned := b.Assignments[segmentID]; assigned {
		return Invalid("segmentId", "片段已分配")
	}
	b.Assignments[segmentID] = []string{first, second}
	b.State = StateAnnotating
	b.Version++
	return nil
}

func (b *ReleaseBatch) AllAssigned() bool {
	if len(b.Assignments) != len(b.Segments) {
		return false
	}
	for id := range b.Segments {
		if len(b.Assignments[id]) != 2 {
			return false
		}
	}
	return true
}
