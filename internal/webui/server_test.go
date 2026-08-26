package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"acousticverdictworkbench/internal/application"
	"acousticverdictworkbench/internal/repository"
)

func TestPageAndSecurityHeaders(t *testing.T) {
	repo, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	handler := New(application.New(repo)).Handler()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("页面或安全头无效：%d", response.Code)
	}
	if len(response.Body.Bytes()) < 500 {
		t.Fatal("HTML 页面为空")
	}
}

func TestBulkClipRouteReturnsAllRowsAndKeepsVersionOnFailure(t *testing.T) {
	repo, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	now := time.Date(2026, 8, 26, 7, 0, 0, 0, time.UTC)
	sequence := 0
	service := application.NewConfigured(repo, func() time.Time { return now }, func(prefix string) string {
		sequence++
		return prefix + "-generated"
	})
	ctx := context.Background()
	created, err := service.CreateBatch(ctx, application.CreateBatchCommand{Metadata: application.Metadata{ActorID: "manager", Role: "manager", IdempotencyKey: "create"}, Title: "HTTP 批量测试", SiteCode: "SITE", RecordingStart: now, RecordingEnd: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	configured, err := service.ConfigureScope(ctx, application.ConfigureScopeCommand{Metadata: application.Metadata{ActorID: "manager", Role: "manager", ExpectedVersion: created.Version, IdempotencyKey: "scope"}, BatchID: created.ID, Title: created.Title, SiteCode: created.SiteCode, RecordingStart: now, RecordingEnd: now.Add(time.Hour), AllowedSpeciesCodes: []string{"BIRD_A"}})
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"actorId": "manager", "role": "manager", "expectedVersion": configured.Version, "clips": []map[string]any{{"id": "clip-a", "sourceName": "a.wav", "capturedAt": now.Add(time.Minute), "durationMs": 1000, "contentSHA256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "sequence": 1}, {"id": "clip-b", "sourceName": "b.wav", "capturedAt": now.Add(2 * time.Hour), "durationMs": 1000, "contentSHA256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "sequence": 1}}}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/batches/"+created.ID+"/clips/bulk", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "invalid-bulk")
	response := httptest.NewRecorder()
	New(service).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("批量错误状态码：%d %s", response.Code, response.Body.String())
	}
	var failure struct {
		Errors []struct {
			Row   int    `json:"row"`
			Field string `json:"field"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil {
		t.Fatal(err)
	}
	if len(failure.Errors) != 3 || failure.Errors[0].Row == 0 {
		t.Fatalf("逐行错误响应不完整：%s", response.Body.String())
	}
	after, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Version != configured.Version || len(after.Clips) != 0 {
		t.Fatalf("HTTP 预检失败后发生写入：version=%d clips=%d", after.Version, len(after.Clips))
	}
}
