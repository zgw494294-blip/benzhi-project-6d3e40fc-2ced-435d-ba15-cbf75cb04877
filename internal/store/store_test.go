package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"voice-clarity-acceptance/internal/domain"
)

func TestCommitRecoversSnapshotAuditAndIdempotency(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	c := &domain.AcceptanceCase{ID: "case-1", CaseNumber: "NO-1", SiteName: "建筑", ResponsibleEngineer: "工程师", Status: domain.StatusDraft, Version: 1, CreatedAt: now, UpdatedAt: now}
	if _, err = s.Commit(CommitRequest{Case: c, Operation: "create", IdempotencyKey: "key-1", Actor: "tester", EventType: "case.created", Response: c, Details: map[string]string{"number": "NO-1"}}); err != nil {
		t.Fatal(err)
	}
	recovered, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := recovered.GetCase(c.ID)
	if err != nil || got.CaseNumber != c.CaseNumber {
		t.Fatalf("恢复验收案失败: %#v %v", got, err)
	}
	if prior, ok := recovered.FindIdempotency("key-1"); !ok || prior.Version != 1 {
		t.Fatalf("幂等结果未恢复: %#v %v", prior, ok)
	}
	if err = recovered.VerifyAudit(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenRejectsBrokenAuditChain(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "audit.jsonl"), []byte(`{"sequence":2}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("损坏的审计序号应阻止恢复")
	}
}
