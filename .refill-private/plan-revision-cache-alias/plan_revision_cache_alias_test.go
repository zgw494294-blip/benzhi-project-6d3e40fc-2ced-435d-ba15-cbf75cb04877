package plan_revision_cache_alias_test

import (
	"testing"
	"time"

	"voice-clarity-acceptance/internal/domain"
	"voice-clarity-acceptance/internal/store"
	"voice-clarity-acceptance/internal/workflow"
)

func TestPlanRevisionCacheDoesNotLeakCallerMutation(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	now := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	c := &domain.AcceptanceCase{
		ID: "case-cache-alias", CaseNumber: "VCA-CACHE-001", SiteName: "缓存边界测试楼",
		Status: domain.StatusDraft, Version: 1, CreatedAt: now, UpdatedAt: now,
		PlanRevisions: []domain.PlanRevisionSnapshot{{
			Revision: 1,
			Points: []domain.MeasurementPoint{{
				ID: "point-1", ZoneID: "zone-1", PointCode: "P-001",
			}},
		}},
	}
	_, err = db.Commit(store.CommitRequest{
		Case: c, Operation: "fixture", IdempotencyKey: "fixture-cache-alias",
		Actor: "private-test", EventType: "case.fixture_created",
	})
	if err != nil {
		t.Fatalf("写入测试案卷失败: %v", err)
	}

	service := workflow.New(db)
	first, err := service.PlanRevisions(c.ID)
	if err != nil {
		t.Fatalf("首次查询计划修订失败: %v", err)
	}
	first[0].Points[0].PointCode = "CALLER-MUTATED"

	second, err := service.PlanRevisions(c.ID)
	if err != nil {
		t.Fatalf("再次查询计划修订失败: %v", err)
	}
	if got := second[0].Points[0].PointCode; got != "P-001" {
		t.Fatalf("缓存复用泄漏了调用方修改: got %q, want %q", got, "P-001")
	}
}
