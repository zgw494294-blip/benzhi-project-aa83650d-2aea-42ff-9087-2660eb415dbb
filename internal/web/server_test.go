package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"phonemereleasedesk/internal/application"
	"phonemereleasedesk/internal/persistence"
)

func TestWorkbenchAndCreateRoute(t *testing.T) {
	repo, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	server := httptest.NewServer(New(application.New(repo)).Handler())
	defer server.Close()
	res, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("工作台响应异常：%d", res.StatusCode)
	}
	res.Body.Close()
	body := `{"id":"web-batch","dialectSite":"测试点","phoneticSystem":"IPA","allowedLabels":["a"],"minimumSegmentMillis":100,"requireDual":true,"actorId":"m","role":"manager"}`
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/batches", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "create-1")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("创建路由返回 %d", res.StatusCode)
	}
}
