package idempotencycrosscase

import (
	"testing"
	"voice-clarity-acceptance/internal/store"
	"voice-clarity-acceptance/internal/workflow"
)

func newCase(t *testing.T, s *workflow.Service, number, zoneID, key string) *workflowCase {
	t.Helper()
	c, err := s.CreateCase(workflow.CreateCaseCommand{
		CaseNumber: number, SiteName: number, ResponsibleEngineer: "工程师", IdempotencyKey: key,
		Zones: []workflow.ZoneInput{{ID: zoneID, Name: number + "区域", UsageClass: "大厅", AreaSquareMeters: 100, MinimumPointCount: 1, IntelligibilityThreshold: .6}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return &workflowCase{id: c.ID, version: c.Version, zoneID: zoneID}
}

type workflowCase struct {
	id      string
	version int64
	zoneID  string
}

func TestIdempotencyKeyCannotReplayAcrossCases(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := workflow.New(st)
	first := newCase(t, s, "CASE-A", "zone-a", "create-a")
	second := newCase(t, s, "CASE-B", "zone-b", "create-b")

	if _, err := s.UpdateScope(workflow.UpdateScopeCommand{
		CaseID: first.id, ExpectedVersion: first.version, IdempotencyKey: "shared-update", SiteName: "更新 A", ResponsibleEngineer: "工程师",
		Zones: []workflow.ZoneInput{{ID: first.zoneID, Name: "A 区域", UsageClass: "大厅", AreaSquareMeters: 100, MinimumPointCount: 1, IntelligibilityThreshold: .6}},
	}); err != nil {
		t.Fatal(err)
	}
	replayed, err := s.UpdateScope(workflow.UpdateScopeCommand{
		CaseID: second.id, ExpectedVersion: second.version, IdempotencyKey: "shared-update", SiteName: "更新 B", ResponsibleEngineer: "工程师",
		Zones: []workflow.ZoneInput{{ID: second.zoneID, Name: "B 区域", UsageClass: "大厅", AreaSquareMeters: 100, MinimumPointCount: 1, IntelligibilityThreshold: .6}},
	})
	if err != nil {
		t.Fatalf("第二个案卷不应复用第一个案卷的幂等结果: %v", err)
	}
	if replayed == nil || replayed.ID != second.id {
		t.Fatalf("幂等结果错误地指向第一个案卷: got=%v want=%s", replayed, second.id)
	}
}
