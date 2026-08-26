package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"acousticverdictworkbench/internal/domain"
)

type checkClient struct {
	base     string
	client   *http.Client
	sequence int
}

func runSelfCheck(ctx context.Context, base string) error {
	c := &checkClient{base: base, client: &http.Client{Timeout: 4 * time.Second}}
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/", nil)
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("声学裁决工作台")) {
		return fmt.Errorf("工作台页面不可用")
	}
	now := time.Now().UTC().Truncate(time.Second)
	batch := domain.ReviewBatch{}
	if err := c.write(ctx, http.MethodPost, "/api/batches", map[string]any{"actorId": "manager-check", "role": "manager", "expectedVersion": 0, "title": "自检声学批次", "siteCode": "SITE-CHECK", "recordingStart": now, "recordingEnd": now.Add(time.Hour)}, &batch, http.StatusCreated); err != nil {
		return err
	}
	path := "/api/batches/" + batch.ID
	if err := c.write(ctx, http.MethodPut, path+"/scope", map[string]any{"actorId": "manager-check", "role": "manager", "expectedVersion": batch.Version, "title": batch.Title, "siteCode": batch.SiteCode, "recordingStart": now, "recordingEnd": now.Add(time.Hour), "allowedSpeciesCodes": []string{"BIRD_A", "BIRD_B"}}, &batch, http.StatusOK); err != nil {
		return err
	}
	if err := c.write(ctx, http.MethodPost, path+"/clips", map[string]any{"actorId": "manager-check", "role": "manager", "expectedVersion": batch.Version, "id": "clip-check", "sourceName": "field-001.wav", "capturedAt": now.Add(time.Minute), "durationMs": 5000, "contentSHA256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "sequence": 1}, &batch, http.StatusCreated); err != nil {
		return err
	}
	if err := c.write(ctx, http.MethodPost, path+"/freeze", map[string]any{"actorId": "manager-check", "role": "manager", "expectedVersion": batch.Version}, &batch, http.StatusOK); err != nil {
		return err
	}
	eventsA := []map[string]any{{"speciesCode": "BIRD_A", "startMs": 500, "endMs": 1800, "confidence": "high", "evidenceNote": "连续三节上扬鸣声"}}
	if err := c.write(ctx, http.MethodPut, path+"/clips/clip-check/draft", map[string]any{"actorId": "ann-a", "role": "annotator", "expectedVersion": batch.Version, "annotatorId": "ann-a", "round": 1, "events": eventsA}, &batch, http.StatusOK); err != nil {
		return err
	}
	if err := c.write(ctx, http.MethodPost, path+"/clips/clip-check/submit", map[string]any{"actorId": "ann-a", "role": "annotator", "expectedVersion": batch.Version, "annotatorId": "ann-a", "round": 1, "confirmed": true}, &batch, http.StatusOK); err != nil {
		return err
	}
	eventsB := []map[string]any{{"speciesCode": "BIRD_B", "startMs": 600, "endMs": 1700, "confidence": "medium", "evidenceNote": "声谱主峰近似但物种判断不同"}}
	if err := c.write(ctx, http.MethodPut, path+"/clips/clip-check/draft", map[string]any{"actorId": "ann-b", "role": "annotator", "expectedVersion": batch.Version, "annotatorId": "ann-b", "round": 1, "events": eventsB}, &batch, http.StatusOK); err != nil {
		return err
	}
	if err := c.write(ctx, http.MethodPost, path+"/clips/clip-check/submit", map[string]any{"actorId": "ann-b", "role": "annotator", "expectedVersion": batch.Version, "annotatorId": "ann-b", "round": 1, "confirmed": true}, &batch, http.StatusOK); err != nil {
		return err
	}
	if batch.Status != domain.BatchAdjudicating || len(batch.OpenDisputes()) != 1 {
		return fmt.Errorf("双标冲突未进入仲裁队列")
	}
	dispute := batch.OpenDisputes()[0]
	if err := c.write(ctx, http.MethodPost, path+"/disputes/"+dispute.ID+"/resolve", map[string]any{"actorId": "reviewer-check", "role": "reviewer", "expectedVersion": batch.Version, "reviewerId": "reviewer-check", "resolution": map[string]any{"kind": "accept_left", "reason": "复听后确认左方物种代码"}}, &batch, http.StatusOK); err != nil {
		return err
	}
	if err := c.write(ctx, http.MethodPost, path+"/quality", map[string]any{"actorId": "reviewer-check", "role": "reviewer", "expectedVersion": batch.Version}, &batch, http.StatusOK); err != nil {
		return err
	}
	if batch.LastQuality == nil || !batch.LastQuality.Passed {
		return fmt.Errorf("质量门禁未通过：%+v", batch.LastQuality)
	}
	if err := c.write(ctx, http.MethodPost, path+"/release", map[string]any{"actorId": "publisher-check", "role": "release_manager", "expectedVersion": batch.Version, "releasedBy": "publisher-check"}, &batch, http.StatusOK); err != nil {
		return err
	}
	if batch.Status != domain.BatchReleased || batch.Manifest == nil || len(batch.Manifest.ManifestSHA256) != 64 || len(batch.Manifest.NormalizedEvents) != 1 {
		return fmt.Errorf("发布清单不完整")
	}
	var conflict map[string]any
	if err := c.write(ctx, http.MethodPost, path+"/quality", map[string]any{"actorId": "reviewer-check", "role": "reviewer", "expectedVersion": batch.Version}, &conflict, http.StatusConflict); err != nil {
		return fmt.Errorf("封存写保护：%w", err)
	}
	return nil
}

func (c *checkClient) write(ctx context.Context, method, path string, payload, target any, expected int) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	c.sequence++
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", fmt.Sprintf("selfcheck-%03d", c.sequence))
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != expected {
		return fmt.Errorf("%s %s 返回 %d，期望 %d：%s", method, path, response.StatusCode, expected, body)
	}
	if len(body) > 0 && target != nil {
		if err := json.Unmarshal(body, target); err != nil {
			return err
		}
	}
	return nil
}
