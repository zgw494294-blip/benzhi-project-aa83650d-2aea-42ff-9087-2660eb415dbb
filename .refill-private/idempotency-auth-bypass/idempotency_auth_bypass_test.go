package idempotencyauthbypass_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"acousticverdictworkbench/internal/application"
	"acousticverdictworkbench/internal/repository"
	"acousticverdictworkbench/internal/webui"
)

func TestIdempotentReplayMustReauthorizeActor(t *testing.T) {
	dataDir := t.TempDir()
	store, err := repository.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	handler := webui.New(application.New(store)).Handler()
	valid := httptest.NewRequest(http.MethodPost, "/api/batches", strings.NewReader(`{
		"actorId":"manager-a",
		"role":"manager",
		"title":"受保护的迁徙样区",
		"siteCode":"PRIVATE-SITE",
		"recordingStart":"2026-08-26T01:00:00Z",
		"recordingEnd":"2026-08-26T02:00:00Z"
	}`))
	valid.Header.Set("Content-Type", "application/json")
	valid.Header.Set("Idempotency-Key", "persisted-create-key")
	validResponse := httptest.NewRecorder()
	handler.ServeHTTP(validResponse, valid)
	if validResponse.Code != http.StatusCreated {
		t.Fatalf("合法建批失败：status=%d body=%s", validResponse.Code, validResponse.Body.String())
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := repository.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	handler = webui.New(application.New(reopened)).Handler()
	unauthorized := httptest.NewRequest(http.MethodPost, "/api/batches", strings.NewReader(`{
		"actorId":"annotator-b",
		"role":"annotator",
		"title":"无权重放",
		"siteCode":"OTHER-SITE",
		"recordingStart":"2026-08-26T03:00:00Z",
		"recordingEnd":"2026-08-26T04:00:00Z"
	}`))
	unauthorized.Header.Set("Content-Type", "application/json")
	unauthorized.Header.Set("Idempotency-Key", "persisted-create-key")
	unauthorizedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedResponse, unauthorized)

	if unauthorizedResponse.Code != http.StatusForbidden {
		t.Fatalf("持久化幂等重放绕过重新授权：status=%d body=%s", unauthorizedResponse.Code, unauthorizedResponse.Body.String())
	}
}
