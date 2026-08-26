package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"phonemereleasedesk/internal/domain"
)

type checkClient struct {
	base    string
	client  *http.Client
	nonce   string
	counter int
}

func (c *checkClient) post(ctx context.Context, path string, body any, target any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	c.counter++
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", fmt.Sprintf("selfcheck-%s-%d", c.nonce, c.counter))
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("POST %s 返回 %d：%s", path, res.StatusCode, data)
	}
	return json.NewDecoder(res.Body).Decode(target)
}

func (c *checkClient) get(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s 返回 %d", path, res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(target)
}

func runSelfCheck(ctx context.Context, base string) error {
	batchID := fmt.Sprintf("selfcheck-%d", time.Now().UnixNano())
	c := &checkClient{base: base, nonce: batchID, client: &http.Client{Timeout: 4 * time.Second}}
	var batch domain.ReleaseBatch
	create := map[string]any{"id": batchID, "dialectSite": "自检方言点", "phoneticSystem": "IPA", "allowedLabels": []string{"a", "i"}, "minimumSegmentMillis": 100, "requireDual": true, "actorId": "manager-selfcheck", "role": "manager"}
	if err := c.post(ctx, "/api/batches", create, &batch); err != nil {
		return err
	}
	segments := []map[string]any{{"id": "seg-1", "sourceRef": "audio-selfcheck", "startMillis": int64(0), "endMillis": int64(1000), "speakerCode": "SPK-SC", "promptText": "自检语料"}}
	var preflight domain.SegmentPreflight
	if err := c.post(ctx, "/api/batches/"+batchID+"/segments/preflight", map[string]any{"expectedVersion": batch.Version, "segments": segments, "actorId": "manager-selfcheck", "role": "manager"}, &preflight); err != nil {
		return err
	}
	if !preflight.Valid || preflight.ContentDigest == "" {
		return fmt.Errorf("批量片段预检未通过：%+v", preflight.Problems)
	}
	if err := c.post(ctx, "/api/batches/"+batchID+"/segments/bulk", map[string]any{"expectedVersion": batch.Version, "contentDigest": preflight.ContentDigest, "segments": preflight.Segments, "actorId": "manager-selfcheck", "role": "manager"}, &batch); err != nil {
		return err
	}
	if err := c.post(ctx, "/api/batches/"+batchID+"/freeze", meta(batch.Version, "manager-selfcheck", "manager"), &batch); err != nil {
		return err
	}
	plan := []map[string]any{{"segmentId": "seg-1", "firstAnnotatorId": "ann-a", "secondAnnotatorId": "ann-b"}}
	var preview domain.AssignmentPreview
	if err := c.post(ctx, "/api/batches/"+batchID+"/assignments/preview", map[string]any{"expectedVersion": batch.Version, "plan": plan, "actorId": "manager-selfcheck", "role": "manager"}, &preview); err != nil {
		return err
	}
	if !preview.Valid || len(preview.Unassigned) != 0 {
		return fmt.Errorf("批量分配预览异常：%+v", preview)
	}
	if err := c.post(ctx, "/api/batches/"+batchID+"/assignments/bulk", map[string]any{"expectedVersion": batch.Version, "plan": preview.Plan, "actorId": "manager-selfcheck", "role": "manager"}, &batch); err != nil {
		return err
	}
	first := submission(batch.Version, "ann-a", "a")
	if err := c.post(ctx, "/api/batches/"+batchID+"/submissions", first, &batch); err != nil {
		return err
	}
	second := submission(batch.Version, "ann-b", "i")
	if err := c.post(ctx, "/api/batches/"+batchID+"/submissions", second, &batch); err != nil {
		return err
	}
	if err := c.post(ctx, "/api/batches/"+batchID+"/checks", meta(batch.Version, "manager-selfcheck", "manager"), &batch); err != nil {
		return err
	}
	if len(batch.Decisions) != 1 || batch.State != domain.StateAdjudicating {
		return fmt.Errorf("检查未生成预期裁定队列")
	}
	var history map[string]any
	if err := c.get(ctx, "/api/batches/"+batchID+"/checks/history", &history); err != nil {
		return err
	}
	decision := map[string]any{"segmentId": "seg-1", "intervalKey": "0-1000", "resolvedLabel": "a", "reason": "自检裁定选择第一份", "adjudicatorId": "judge-selfcheck"}
	if err := c.post(ctx, "/api/batches/"+batchID+"/decisions/bulk", map[string]any{"expectedVersion": batch.Version, "decisions": []any{decision}, "actorId": "judge-selfcheck", "role": "adjudicator"}, &batch); err != nil {
		return err
	}
	repair := map[string]any{"expectedVersion": batch.Version, "segmentId": "seg-1", "rule": "dual-agreement", "targetKind": "annotation", "annotatorId": "ann-a", "reason": "自检定向返修", "reviewerId": "reviewer-selfcheck", "actorId": "reviewer-selfcheck", "role": "reviewer"}
	if err := c.post(ctx, "/api/batches/"+batchID+"/repairs", repair, &batch); err != nil {
		return err
	}
	var tasks map[string]any
	if err := c.get(ctx, "/api/batches/"+batchID+"/repairs/tasks?role=annotator&actorId=ann-a", &tasks); err != nil {
		return err
	}
	revised := submission(batch.Version, "ann-a", "i")
	if err := c.post(ctx, "/api/batches/"+batchID+"/submissions", revised, &batch); err != nil {
		return err
	}
	if batch.State != domain.StateCandidate || len(batch.VerificationRuns) < 2 {
		return fmt.Errorf("定向返修后未自动完成受影响规则重检：state=%s runs=%d repairs=%+v", batch.State, len(batch.VerificationRuns), batch.Repairs)
	}
	before, after := batch.VerificationRuns[len(batch.VerificationRuns)-2].ID, batch.VerificationRuns[len(batch.VerificationRuns)-1].ID
	var comparison map[string]any
	if err := c.get(ctx, "/api/batches/"+batchID+"/checks/compare?beforeRunId="+before+"&afterRunId="+after, &comparison); err != nil {
		return err
	}
	seal := map[string]any{"expectedVersion": batch.Version, "reviewerId": "reviewer-selfcheck", "actorId": "reviewer-selfcheck", "role": "reviewer"}
	if err := c.post(ctx, "/api/batches/"+batchID+"/seal", seal, &batch); err != nil {
		return err
	}
	if batch.Credential == nil || batch.State != domain.StateSealed {
		return fmt.Errorf("复核封存未签发凭据")
	}
	var detail map[string]any
	if err := c.get(ctx, "/api/credentials?batchId="+batchID+"&page=1&pageSize=10", &detail); err != nil {
		return err
	}
	var dimensions struct {
		Valid bool `json:"valid"`
	}
	if err := c.get(ctx, "/api/credentials/verify?batchId="+batchID, &dimensions); err != nil {
		return err
	}
	if !dimensions.Valid {
		return fmt.Errorf("发布凭据分项核验失败")
	}
	var verification struct {
		Valid      bool   `json:"valid"`
		Stored     string `json:"storedDigest"`
		Recomputed string `json:"recomputedDigest"`
	}
	if err := c.get(ctx, "/api/credentials/"+batch.Credential.ID+"/verify", &verification); err != nil {
		return err
	}
	if !verification.Valid || verification.Stored == "" || verification.Stored != verification.Recomputed {
		return fmt.Errorf("发布凭据摘要核验失败")
	}
	return nil
}

func meta(version uint64, actor, role string) map[string]any {
	return map[string]any{"expectedVersion": version, "actorId": actor, "role": role}
}

func submission(version uint64, actor, label string) map[string]any {
	return map[string]any{"expectedVersion": version, "segmentId": "seg-1", "annotatorId": actor, "intervals": []map[string]any{{"startMillis": 0, "endMillis": 1000, "label": label}}, "submit": true, "actorId": actor, "role": "annotator"}
}
