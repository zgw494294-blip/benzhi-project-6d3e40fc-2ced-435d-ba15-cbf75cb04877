package auditcommitrollback_test

import (
	"os"
	"path/filepath"
	"testing"

	"voice-clarity-acceptance/internal/domain"
	"voice-clarity-acceptance/internal/store"
)

func TestCommitFailureDoesNotCorruptAuditChain(t *testing.T) {
	dir := t.TempDir()
	repository, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	caseData := &domain.AcceptanceCase{ID: "case-rollback", CaseNumber: "ROLLBACK-1", Version: 1}
	request := store.CommitRequest{
		Case:           caseData,
		Operation:      "create_case",
		IdempotencyKey: "rollback-key",
		EventType:      "case.created",
	}

	// 将快照路径设为目录，使审计追加后原子重命名失败，模拟可恢复的资源失效。
	snapshotPath := filepath.Join(dir, "snapshot.json")
	if err := os.Mkdir(snapshotPath, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Commit(request); err == nil {
		t.Fatal("快照资源失效时提交应返回错误")
	}
	if err := os.RemoveAll(snapshotPath); err != nil {
		t.Fatal(err)
	}

	if _, err := repository.Commit(request); err != nil {
		t.Fatalf("恢复快照资源后重试不应失败: %v", err)
	}
	recovered, err := store.Open(dir)
	if err != nil {
		t.Fatalf("重启恢复不应因一次失败提交而失败: %v", err)
	}
	if err := recovered.VerifyAudit(); err != nil {
		t.Fatalf("失败提交后的审计链不应损坏: %v", err)
	}
}
