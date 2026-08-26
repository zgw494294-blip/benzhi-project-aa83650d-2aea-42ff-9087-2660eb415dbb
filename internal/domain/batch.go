package domain

import (
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

func NewReviewBatch(id, title, site string, start, end, now time.Time) (*ReviewBatch, error) {
	title = strings.TrimSpace(title)
	site = strings.TrimSpace(site)
	if id == "" || title == "" || site == "" {
		return nil, Invalid("batch", "批次 ID、标题和采集地点均不能为空")
	}
	if len([]rune(title)) > 120 || len([]rune(site)) > 64 {
		return nil, Invalid("batch", "批次标题或地点过长")
	}
	if !end.After(start) {
		return nil, Invalid("recordingEnd", "录音结束时间必须晚于开始时间")
	}
	b := &ReviewBatch{ID: id, Title: title, SiteCode: site, RecordingStart: start.UTC(), RecordingEnd: end.UTC(), Status: BatchDraft, CreatedAt: now.UTC(), Clips: []AudioClip{}, Submissions: []AnnotationSubmission{}, Disputes: []DisputeCase{}, AdjudicationTrail: []AdjudicationRecord{}}
	b.record("batch.created", now, map[string]any{"title": title, "siteCode": site})
	return b, nil
}

func (b *ReviewBatch) ConfigureScope(title, site string, start, end time.Time, species []string, now time.Time) error {
	if b.Status != BatchDraft {
		return ErrStateConflict
	}
	title, site = strings.TrimSpace(title), strings.TrimSpace(site)
	if title == "" || site == "" || !end.After(start) {
		return Invalid("scope", "标题、地点和有效录音时间范围均为必填")
	}
	codes, err := NormalizeSpecies(species)
	if err != nil {
		return err
	}
	b.Title, b.SiteCode = title, site
	b.RecordingStart, b.RecordingEnd = start.UTC(), end.UTC()
	b.AllowedSpeciesCodes = codes
	b.record("batch.scope_configured", now, map[string]any{"speciesCount": len(codes)})
	return nil
}

func (b *ReviewBatch) AddClip(clip AudioClip, now time.Time) error {
	if b.Status != BatchDraft {
		return ErrStateConflict
	}
	normalized, errors := b.validateClips([]AudioClip{clip})
	if len(errors) > 0 {
		return Invalid(errors[0].Field, errors[0].Message)
	}
	b.Clips = append(b.Clips, normalized[0])
	sort.Slice(b.Clips, func(i, j int) bool { return b.Clips[i].Sequence < b.Clips[j].Sequence })
	b.record("clip.added", now, map[string]any{"clipId": normalized[0].ID, "sequence": normalized[0].Sequence})
	return nil
}

func (b *ReviewBatch) AddClips(clips []AudioClip, now time.Time) error {
	if b.Status != BatchDraft {
		return ErrStateConflict
	}
	if len(clips) == 0 {
		return Invalid("clips", "至少需要一条录音片段")
	}
	normalized, errors := b.validateClips(clips)
	if len(errors) > 0 {
		return InvalidFields(errors)
	}
	b.Clips = append(b.Clips, normalized...)
	sort.Slice(b.Clips, func(i, j int) bool {
		if b.Clips[i].Sequence != b.Clips[j].Sequence {
			return b.Clips[i].Sequence < b.Clips[j].Sequence
		}
		return b.Clips[i].ID < b.Clips[j].ID
	})
	ids := make([]string, len(normalized))
	sequences := make([]int, len(normalized))
	for i := range normalized {
		ids[i], sequences[i] = normalized[i].ID, normalized[i].Sequence
	}
	b.record("clip.bulk_registered", now, map[string]any{"addedCount": len(normalized), "clipIds": ids, "sequences": sequences})
	return nil
}

func (b *ReviewBatch) validateClips(clips []AudioClip) ([]AudioClip, []FieldError) {
	existingIDs := make(map[string]struct{}, len(b.Clips))
	existingSequences := make(map[int]struct{}, len(b.Clips))
	existingDigests := make(map[string]struct{}, len(b.Clips))
	for _, clip := range b.Clips {
		existingIDs[clip.ID] = struct{}{}
		existingSequences[clip.Sequence] = struct{}{}
		existingDigests[strings.ToLower(clip.ContentSHA256)] = struct{}{}
	}
	normalized := make([]AudioClip, len(clips))
	errors := make([]FieldError, 0)
	for i, raw := range clips {
		row := i + 1
		clip := raw
		clip.ID = strings.TrimSpace(clip.ID)
		clip.SourceName = strings.TrimSpace(clip.SourceName)
		clip.ContentSHA256 = strings.ToLower(strings.TrimSpace(clip.ContentSHA256))
		clip.CapturedAt = clip.CapturedAt.UTC()
		clip.BatchID, clip.ReviewState = b.ID, "pending"
		if clip.ID == "" || len(clip.ID) > 128 {
			errors = append(errors, FieldError{Row: row, Field: "id", Message: "片段标识不能为空且不能超过 128 字符"})
		} else if _, exists := existingIDs[clip.ID]; exists {
			errors = append(errors, FieldError{Row: row, Field: "id", Message: "片段标识重复"})
		} else {
			existingIDs[clip.ID] = struct{}{}
		}
		if clip.SourceName == "" || len([]rune(clip.SourceName)) > 200 {
			errors = append(errors, FieldError{Row: row, Field: "sourceName", Message: "来源名称不能为空且不能超过 200 个字符"})
		}
		if clip.CapturedAt.IsZero() || clip.CapturedAt.Before(b.RecordingStart) || clip.CapturedAt.After(b.RecordingEnd) {
			errors = append(errors, FieldError{Row: row, Field: "capturedAt", Message: "片段采集时间必须位于批次时间范围内"})
		}
		if clip.DurationMs <= 0 || clip.DurationMs > 24*60*60*1000 {
			errors = append(errors, FieldError{Row: row, Field: "durationMs", Message: "片段时长必须为不超过 24 小时的正毫秒数"})
		}
		decoded, digestErr := hex.DecodeString(clip.ContentSHA256)
		if digestErr != nil || len(decoded) != 32 {
			errors = append(errors, FieldError{Row: row, Field: "contentSHA256", Message: "内容摘要必须是 64 位 SHA-256 十六进制文本"})
		} else if _, exists := existingDigests[clip.ContentSHA256]; exists {
			errors = append(errors, FieldError{Row: row, Field: "contentSHA256", Message: "内容摘要重复"})
		} else {
			existingDigests[clip.ContentSHA256] = struct{}{}
		}
		if clip.Sequence <= 0 {
			errors = append(errors, FieldError{Row: row, Field: "sequence", Message: "片段序号必须为正整数"})
		} else if _, exists := existingSequences[clip.Sequence]; exists {
			errors = append(errors, FieldError{Row: row, Field: "sequence", Message: "片段序号重复"})
		} else {
			existingSequences[clip.Sequence] = struct{}{}
		}
		normalized[i] = clip
	}
	return normalized, errors
}

func (b *ReviewBatch) RemoveClip(id string, now time.Time) error {
	if b.Status != BatchDraft {
		return ErrStateConflict
	}
	for i := range b.Clips {
		if b.Clips[i].ID == id {
			b.Clips = append(b.Clips[:i], b.Clips[i+1:]...)
			b.record("clip.removed", now, map[string]any{"clipId": id})
			return nil
		}
	}
	return ErrNotFound
}

func (b *ReviewBatch) Freeze(now time.Time) error {
	if b.Status != BatchDraft {
		return ErrStateConflict
	}
	if len(b.Clips) == 0 || len(b.AllowedSpeciesCodes) == 0 {
		return Invalid("scope", "冻结前必须登记片段和允许物种范围")
	}
	b.Status = BatchFrozen
	t := now.UTC()
	b.FrozenAt = &t
	b.record("batch.frozen", now, map[string]any{"clipCount": len(b.Clips)})
	return nil
}

func (b *ReviewBatch) Clip(id string) (*AudioClip, error) {
	for i := range b.Clips {
		if b.Clips[i].ID == id {
			return &b.Clips[i], nil
		}
	}
	return nil, ErrNotFound
}

func (b *ReviewBatch) SpeciesSet() map[string]struct{} {
	result := make(map[string]struct{}, len(b.AllowedSpeciesCodes))
	for _, code := range b.AllowedSpeciesCodes {
		result[code] = struct{}{}
	}
	return result
}
