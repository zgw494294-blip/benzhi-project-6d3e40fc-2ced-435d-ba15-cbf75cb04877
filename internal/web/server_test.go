package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"voice-clarity-acceptance/internal/store"
	"voice-clarity-acceptance/internal/workflow"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(workflow.New(s))
}

func TestWorkbenchAndHealthAreServed(t *testing.T) {
	h := testHandler(t)
	page := httptest.NewRecorder()
	h.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "<body>") || !strings.Contains(page.Body.String(), "语音可懂度验收工作台") {
		t.Fatalf("工作台页面无效: %d %s", page.Code, page.Body.String())
	}
	health := httptest.NewRecorder()
	h.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if health.Code != http.StatusOK || health.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("健康检查或安全头无效: %d %#v", health.Code, health.Header())
	}
}

func TestStructuredValidationError(t *testing.T) {
	h := testHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/cases", strings.NewReader(`{"caseNumber":"","idempotencyKey":"x","zones":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "validation_failed") {
		t.Fatalf("应返回结构化字段错误: %d %s", rec.Code, rec.Body.String())
	}
}
