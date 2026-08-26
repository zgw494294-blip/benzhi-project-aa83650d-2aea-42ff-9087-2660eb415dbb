package domain

import (
	"sort"
	"strings"
	"time"
)

func (b *ReviewBatch) SaveDraft(id, clipID, annotator string, round int, events []CandidateEvent, reason string, now time.Time) error {
	if b.Status == BatchDraft || b.Status == BatchReleased {
		return ErrStateConflict
	}
	clip, err := b.Clip(clipID)
	if err != nil {
		return err
	}
	annotator = strings.TrimSpace(annotator)
	if annotator == "" {
		return Invalid("annotatorId", "标注员 ID 不能为空")
	}
	if round < 1 {
		return Invalid("round", "标注轮次必须从 1 开始")
	}
	current, exists := b.submission(clipID, annotator, round)
	task, taskErr := b.taskForEdit(clipID, annotator, round)
	if taskErr != nil {
		return taskErr
	}
	if exists && current.Status == SubmissionSubmitted {
		return ErrStateConflict
	}
	if round > 1 && task == nil {
		return ErrTaskRound
	}
	if round == 1 && !exists {
		for _, submission := range b.Submissions {
			if submission.ClipID == clipID && submission.AnnotatorID == annotator {
				return ErrStateConflict
			}
		}
	}
	if exists {
		id = current.ID
	}
	normalized, err := NormalizeCandidates(events, id, b.SpeciesSet(), clip.DurationMs)
	if err != nil {
		return err
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].StartMs != normalized[j].StartMs {
			return normalized[i].StartMs < normalized[j].StartMs
		}
		if normalized[i].EndMs != normalized[j].EndMs {
			return normalized[i].EndMs < normalized[j].EndMs
		}
		return normalized[i].ID < normalized[j].ID
	})
	if exists {
		current.Events, current.RevisionReason, current.Status = normalized, strings.TrimSpace(reason), SubmissionDraft
	} else {
		b.Submissions = append(b.Submissions, AnnotationSubmission{ID: id, ClipID: clipID, AnnotatorID: annotator, Round: round, Status: SubmissionDraft, Events: normalized, RevisionReason: strings.TrimSpace(reason)})
	}
	if task != nil && task.Status == ReannotationPending {
		task.Status = ReannotationInProgress
	}
	if b.Status == BatchFrozen {
		b.Status = BatchAnnotating
	}
	b.record("annotation.draft_saved", now, map[string]any{"clipId": clipID, "annotatorId": annotator, "round": round, "eventCount": len(events)})
	return nil
}

func (b *ReviewBatch) Submit(clipID, annotator string, round int, now time.Time) error {
	return b.SubmitConfirmed(clipID, annotator, round, "", true, now)
}

func (b *ReviewBatch) SubmitConfirmed(clipID, annotator string, round int, revisionReason string, confirmed bool, now time.Time) error {
	if !confirmed {
		return Invalid("confirmed", "提交前必须确认事件数量与区间摘要")
	}
	sub, ok := b.submission(clipID, annotator, round)
	if !ok {
		return ErrNotFound
	}
	if sub.Status == SubmissionSubmitted {
		if round > 1 {
			return ErrTaskClosed
		}
		return ErrStateConflict
	}
	if sub.Status != SubmissionDraft && sub.Status != SubmissionReopened {
		return ErrStateConflict
	}
	var task *ReannotationTask
	if round > 1 {
		var err error
		task, err = b.taskForEdit(clipID, annotator, round)
		if err != nil {
			return err
		}
		if task == nil {
			return ErrTaskRound
		}
		revisionReason = strings.TrimSpace(revisionReason)
		if revisionReason == "" {
			return Invalid("revisionReason", "返标提交必须填写修订说明")
		}
		if len([]rune(revisionReason)) > 1000 {
			return Invalid("revisionReason", "修订说明不能超过 1000 个字符")
		}
		sub.RevisionReason = revisionReason
	}
	t := now.UTC()
	sub.Status, sub.SubmittedAt = SubmissionSubmitted, &t
	if task != nil {
		task.Status, task.CompletedAt, task.RevisionReason = ReannotationClosed, &t, revisionReason
	}
	b.record("annotation.submitted", now, map[string]any{"clipId": clipID, "annotatorId": annotator, "round": round})
	return nil
}

func (b *ReviewBatch) submission(clipID, annotator string, round int) (*AnnotationSubmission, bool) {
	for i := range b.Submissions {
		s := &b.Submissions[i]
		if s.ClipID == clipID && s.AnnotatorID == annotator && s.Round == round {
			return s, true
		}
	}
	return nil, false
}

func (b *ReviewBatch) LatestSubmitted(clipID string) []AnnotationSubmission {
	maxByAnnotator := map[string]int{}
	for _, s := range b.Submissions {
		if s.ClipID == clipID && s.Status == SubmissionSubmitted && s.Round > maxByAnnotator[s.AnnotatorID] {
			maxByAnnotator[s.AnnotatorID] = s.Round
		}
	}
	result := []AnnotationSubmission{}
	for _, s := range b.Submissions {
		if s.ClipID == clipID && s.Status == SubmissionSubmitted && s.Round == maxByAnnotator[s.AnnotatorID] {
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].AnnotatorID < result[j].AnnotatorID })
	return result
}

func (b *ReviewBatch) VisibleSubmissions(viewer, role string) []AnnotationSubmission {
	result := make([]AnnotationSubmission, 0, len(b.Submissions))
	for _, s := range b.Submissions {
		if s.Status == SubmissionSubmitted || s.AnnotatorID == viewer {
			copy := s
			copy.Events = append([]CandidateEvent(nil), s.Events...)
			result = append(result, copy)
		}
	}
	return result
}

func (b *ReviewBatch) taskForEdit(clipID, annotator string, round int) (*ReannotationTask, error) {
	var clipTask *ReannotationTask
	for i := range b.ReannotationTasks {
		task := &b.ReannotationTasks[i]
		if task.ClipID != clipID {
			continue
		}
		if task.TargetAnnotator == annotator && task.Round == round && task.Status == ReannotationClosed {
			return nil, ErrTaskClosed
		}
		if task.Status == ReannotationClosed {
			continue
		}
		clipTask = task
		if task.TargetAnnotator != annotator {
			continue
		}
		if task.Round != round {
			return nil, ErrTaskRound
		}
		return task, nil
	}
	if clipTask != nil && clipTask.TargetAnnotator != annotator {
		return nil, ErrTaskOwner
	}
	return nil, nil
}

func (b *ReviewBatch) VisibleReannotationTasks(viewer, role string) []ReannotationTask {
	result := make([]ReannotationTask, 0)
	for _, task := range b.ReannotationTasks {
		if role == "reviewer" || task.TargetAnnotator == viewer {
			result = append(result, task)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Status != result[j].Status {
			return result[i].Status < result[j].Status
		}
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}
