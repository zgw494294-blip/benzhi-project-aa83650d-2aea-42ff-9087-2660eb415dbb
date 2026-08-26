package domain

import (
	"fmt"
	"strings"
	"time"
)

func (b *ReleaseBatch) SaveSubmission(segmentID, annotator string, intervals []PhonemeInterval, submit bool, now time.Time) error {
	if err := b.EnsureWritable(); err != nil {
		return err
	}
	if b.State != StateAnnotating && b.State != StateRepair {
		return ErrInvalidState
	}
	segment, exists := b.Segments[segmentID]
	if !exists {
		return ErrNotFound
	}
	assigned := b.Assignments[segmentID]
	if len(assigned) != 2 || (annotator != assigned[0] && annotator != assigned[1]) {
		return ErrForbidden
	}
	if err := ValidateIntervals(segment, intervals, b); err != nil {
		return err
	}
	list := b.Submissions[segmentID]
	index := -1
	for i := range list {
		if list[i].AnnotatorID == annotator {
			index = i
			break
		}
	}
	if index >= 0 && list[index].Status == SubmissionSubmitted && b.State != StateRepair {
		return Invalid("submission", "已提交标注不可覆盖")
	}
	if b.State == StateRepair && index >= 0 && list[index].Status != SubmissionUnlocked {
		return Invalid("submission", "该标注未被定向解锁")
	}
	status := SubmissionDraft
	var submittedAt *time.Time
	if submit {
		status = SubmissionSubmitted
		instant := now.UTC()
		submittedAt = &instant
	}
	revision := 1
	if index >= 0 {
		revision = list[index].Revision + 1
	}
	value := AnnotationSubmission{ID: fmt.Sprintf("sub-%s-%s", segmentID, annotator), SegmentID: segmentID, AnnotatorID: annotator, Intervals: append([]PhonemeInterval(nil), intervals...), Revision: revision, Status: status, SubmittedAt: submittedAt}
	if index >= 0 {
		list[index] = value
	} else {
		list = append(list, value)
	}
	b.Submissions[segmentID] = list
	if submit {
		b.resolveRepairs("annotation", segmentID, annotator, "", now)
	}
	b.Version++
	return nil
}

func ValidateIntervals(segment RecordingSegment, intervals []PhonemeInterval, batch *ReleaseBatch) error {
	if len(intervals) == 0 {
		return Invalid("intervals", "至少录入一个音素区间")
	}
	expected := segment.StartMillis
	for i, interval := range intervals {
		if strings.TrimSpace(interval.Label) == "" {
			return Invalid("label", "音素标签不能为空")
		}
		if !batch.HasLabel(interval.Label) {
			return Invalid("label", "音素标签不在白名单中")
		}
		if interval.StartMillis != expected {
			return Invalid("intervals", fmt.Sprintf("第 %d 个区间与前项不连续", i+1))
		}
		if interval.EndMillis <= interval.StartMillis {
			return Invalid("intervals", "音素区间终点必须晚于起点")
		}
		if interval.StartMillis < segment.StartMillis || interval.EndMillis > segment.EndMillis {
			return Invalid("intervals", "音素区间超出片段边界")
		}
		expected = interval.EndMillis
	}
	if expected != segment.EndMillis {
		return Invalid("intervals", "音素区间未覆盖片段终点")
	}
	return nil
}

func (b *ReleaseBatch) VisibleSubmission(segmentID, viewer string) (*AnnotationSubmission, error) {
	assigned := b.Assignments[segmentID]
	if viewer != assigned[0] && viewer != assigned[1] {
		return nil, ErrForbidden
	}
	for _, item := range b.Submissions[segmentID] {
		if item.AnnotatorID == viewer {
			copyItem := item
			copyItem.Intervals = append([]PhonemeInterval(nil), item.Intervals...)
			return &copyItem, nil
		}
	}
	return nil, ErrNotFound
}

func (b *ReleaseBatch) AllSubmitted() bool {
	if !b.AllAssigned() {
		return false
	}
	for id := range b.Segments {
		list := b.Submissions[id]
		if len(list) != 2 {
			return false
		}
		if list[0].AnnotatorID == list[1].AnnotatorID || list[0].Status != SubmissionSubmitted || list[1].Status != SubmissionSubmitted {
			return false
		}
	}
	return true
}

func (b *ReleaseBatch) BeginChecking() error {
	if b.State != StateAnnotating && b.State != StateRepair {
		return ErrInvalidState
	}
	if !b.AllSubmitted() {
		return Invalid("submissions", "所有片段必须取得两份独立提交")
	}
	b.State = StateChecking
	b.Version++
	return nil
}
