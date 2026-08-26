package domain

import (
	"strings"
	"time"
)

func (b *ReviewBatch) ReplaceClipDisputes(clipID string, cases []DisputeCase, now time.Time) error {
	if _, err := b.Clip(clipID); err != nil {
		return err
	}
	for i := range b.Disputes {
		if b.Disputes[i].ClipID == clipID && !b.Disputes[i].Superseded {
			b.Disputes[i].Superseded = true
		}
	}
	for _, d := range cases {
		d.ClipID = clipID
		d.Superseded = false
		if d.Kind == DisputeAgreement {
			d.Status = DisputeResolved
		} else {
			d.Status = DisputeOpen
		}
		b.Disputes = append(b.Disputes, d)
	}
	if b.HasOpenDisputes() {
		b.Status = BatchAdjudicating
	} else {
		b.Status = BatchReady
	}
	b.record("matching.recomputed", now, map[string]any{"clipId": clipID, "caseCount": len(cases)})
	return nil
}

func (b *ReviewBatch) ResolveDispute(id, reviewer string, resolution Resolution, recordID string, now time.Time) error {
	taskID, submissionID := recordID+"-task", recordID+"-submission"
	eventIDs := []string{}
	if resolution.Kind == ResolutionReturn {
		latest := 0
		for _, submission := range b.Submissions {
			if submission.ClipID == b.disputeClipID(id) && submission.AnnotatorID == resolution.ReturnAnnotator && submission.Round > latest {
				latest = submission.Round
				eventIDs = make([]string, len(submission.Events))
				for i := range eventIDs {
					eventIDs[i] = recordID + "-event-" + itoaDomain(int64(i+1))
				}
			}
		}
	}
	return b.ResolveDisputeWithTask(id, reviewer, resolution, recordID, taskID, submissionID, eventIDs, now)
}

func (b *ReviewBatch) ResolveDisputeWithTask(id, reviewer string, resolution Resolution, recordID, taskID, submissionID string, eventIDs []string, now time.Time) error {
	if b.Status == BatchReleased {
		return ErrAlreadyReleased
	}
	reviewer, resolution.Reason = strings.TrimSpace(reviewer), strings.TrimSpace(resolution.Reason)
	if reviewer == "" || resolution.Reason == "" {
		return Invalid("resolution", "复核员和仲裁理由均为必填")
	}
	if len([]rune(reviewer)) > 128 || len([]rune(resolution.Reason)) > 1000 {
		return Invalid("resolution", "复核员标识或仲裁理由过长")
	}
	d, err := b.Dispute(id)
	if err != nil {
		return err
	}
	if d.Status != DisputeOpen || d.Superseded {
		return ErrStateConflict
	}
	switch resolution.Kind {
	case ResolutionAcceptLeft:
		if d.LeftEventID == "" {
			return Invalid("resolution", "该分歧没有左方事件")
		}
		resolution.NormalizedEvent = b.candidateByID(d.LeftEventID)
	case ResolutionAcceptRight:
		if d.RightEventID == "" {
			return Invalid("resolution", "该分歧没有右方事件")
		}
		resolution.NormalizedEvent = b.candidateByID(d.RightEventID)
	case ResolutionMerge:
		clip, _ := b.Clip(d.ClipID)
		if resolution.NormalizedEvent == nil {
			return Invalid("normalizedEvent", "合并修订必须提供规范事件")
		}
		n, e := NormalizeCandidate(*resolution.NormalizedEvent, b.SpeciesSet(), clip.DurationMs)
		if e != nil {
			return e
		}
		resolution.NormalizedEvent = &n
	case ResolutionNoEvent:
		resolution.NormalizedEvent = nil
	case ResolutionReturn:
		resolution.ReturnAnnotator = strings.TrimSpace(resolution.ReturnAnnotator)
		if resolution.ReturnAnnotator == "" {
			return Invalid("returnAnnotator", "返标必须指定标注员")
		}
		if !b.disputeInvolvesAnnotator(*d, resolution.ReturnAnnotator) {
			return Invalid("returnAnnotator", "返标目标必须是该分歧关联的标注员")
		}
		if err := b.reopenClip(*d, resolution.ReturnAnnotator, resolution.Reason, taskID, submissionID, eventIDs, now); err != nil {
			return err
		}
	default:
		return Invalid("kind", "不支持的仲裁类型")
	}
	t := now.UTC()
	d.Resolution, d.ReviewerID, d.ResolvedAt = &resolution, reviewer, &t
	if resolution.Kind == ResolutionReturn {
		d.Status = DisputeReturned
		b.Status = BatchAnnotating
	} else {
		d.Status = DisputeResolved
	}
	b.AdjudicationTrail = append(b.AdjudicationTrail, AdjudicationRecord{ID: recordID, DisputeID: id, ClipID: d.ClipID, Kind: resolution.Kind, ReviewerID: reviewer, TargetAnnotator: resolution.ReturnAnnotator, Reason: resolution.Reason, At: t})
	if resolution.Kind != ResolutionReturn && !b.HasOpenDisputes() {
		b.Status = BatchReady
	}
	b.record("dispute.resolved", now, map[string]any{"disputeId": id, "kind": resolution.Kind})
	return nil
}

func (b *ReviewBatch) disputeClipID(id string) string {
	for _, dispute := range b.Disputes {
		if dispute.ID == id {
			return dispute.ClipID
		}
	}
	return ""
}

func (b *ReviewBatch) reopenClip(dispute DisputeCase, annotator, reason, taskID, submissionID string, eventIDs []string, now time.Time) error {
	clipID := dispute.ClipID
	for _, task := range b.ReannotationTasks {
		if task.ClipID == clipID && task.Status != ReannotationClosed {
			return ErrStateConflict
		}
	}
	latest := 0
	for _, s := range b.Submissions {
		if s.ClipID == clipID && s.AnnotatorID == annotator && s.Round > latest {
			latest = s.Round
		}
	}
	if latest == 0 {
		return Invalid("returnAnnotator", "指定标注员在该片段没有提交记录")
	}
	base, _ := b.submission(clipID, annotator, latest)
	copyEvents := append([]CandidateEvent(nil), base.Events...)
	if taskID == "" || submissionID == "" || len(eventIDs) != len(copyEvents) {
		return Invalid("returnTask", "返标任务标识或事件标识不完整")
	}
	for i := range copyEvents {
		copyEvents[i].ID = eventIDs[i]
		copyEvents[i].SubmissionID = submissionID
	}
	nextRound := latest + 1
	b.Submissions = append(b.Submissions, AnnotationSubmission{ID: submissionID, ClipID: clipID, AnnotatorID: annotator, Round: nextRound, Status: SubmissionReopened, Events: copyEvents})
	b.ReannotationTasks = append(b.ReannotationTasks, ReannotationTask{ID: taskID, DisputeID: dispute.ID, ClipID: clipID, TargetAnnotator: annotator, Round: nextRound, Reason: reason, OriginalKind: dispute.Kind, OriginalLeftID: dispute.LeftEventID, OriginalRightID: dispute.RightEventID, OriginalBasis: dispute.Basis, Status: ReannotationPending, CreatedAt: now.UTC()})
	for i := range b.Clips {
		if b.Clips[i].ID == clipID {
			b.Clips[i].ReviewState = "returned"
		}
	}
	return nil
}

func (b *ReviewBatch) disputeInvolvesAnnotator(dispute DisputeCase, annotator string) bool {
	for _, submission := range b.LatestSubmitted(dispute.ClipID) {
		if submission.AnnotatorID != annotator {
			continue
		}
		if dispute.LeftEventID == "" && dispute.RightEventID == "" {
			return true
		}
		for _, event := range submission.Events {
			if event.ID == dispute.LeftEventID || event.ID == dispute.RightEventID {
				return true
			}
		}
		// 单边缺失本身也与该片段的另一位提交者相关。
		if dispute.Kind == DisputeLeftOnly || dispute.Kind == DisputeRightOnly {
			return true
		}
	}
	return false
}

func (b *ReviewBatch) Dispute(id string) (*DisputeCase, error) {
	for i := range b.Disputes {
		if b.Disputes[i].ID == id {
			return &b.Disputes[i], nil
		}
	}
	return nil, ErrNotFound
}

func (b *ReviewBatch) HasOpenDisputes() bool {
	for _, d := range b.Disputes {
		if !d.Superseded && d.Kind != DisputeAgreement && d.Status == DisputeOpen {
			return true
		}
	}
	return false
}

func (b *ReviewBatch) candidateByID(id string) *CandidateEvent {
	for _, s := range b.Submissions {
		for _, e := range s.Events {
			if e.ID == id {
				copy := e
				return &copy
			}
		}
	}
	return nil
}
