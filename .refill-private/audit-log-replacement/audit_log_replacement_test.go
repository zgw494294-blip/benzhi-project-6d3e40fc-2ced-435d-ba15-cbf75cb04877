package auditlogreplacement

import (
	"os"
	"path/filepath"
	"testing"

	"voice-clarity-acceptance/internal/domain"
	"voice-clarity-acceptance/internal/store"
)

func TestAuditLogReplacementDoesNotBreakRestartRecovery(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	c := &domain.AcceptanceCase{ID: "case-rotation", CaseNumber: "ROTATION-1", Version: 1}
	if _, err = s.Commit(store.CommitRequest{
		Case: c, Operation: "create_case", IdempotencyKey: "rotation-create",
		Actor: "tester", EventType: "case.created", Details: map[string]any{"version": 1},
	}); err != nil {
		t.Fatalf("首次提交失败: %v", err)
	}

	auditPath := filepath.Join(dir, "audit.jsonl")
	replacementPath := filepath.Join(dir, "audit.replacement")
	content, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(replacementPath, content, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(replacementPath, auditPath); err != nil {
		t.Fatal(err)
	}

	c.Version = 2
	if _, err = s.Commit(store.CommitRequest{
		Case: c, Operation: "update_scope", IdempotencyKey: "rotation-update",
		Actor: "tester", EventType: "case.scope_updated", Details: map[string]any{"version": 2},
	}); err != nil {
		t.Fatalf("轮换后的提交不应失败: %v", err)
	}

	if _, err = store.Open(dir); err != nil {
		t.Fatalf("审计日志被等价替换后，成功提交的数据应能在重启时恢复: %v", err)
	}
}
