package auditdetailsalias_test

import (
	"testing"

	"voice-clarity-acceptance/internal/store"
	"voice-clarity-acceptance/internal/workflow"
)

func TestAuditQueryDoesNotAllowDigestMutation(t *testing.T) {
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(s)
	caseSnapshot, err := service.CreateCase(workflow.CreateCaseCommand{
		CaseNumber:          "AUDIT-ALIAS-1",
		SiteName:            "审计轨迹测试站",
		ResponsibleEngineer: "测试工程师",
		IdempotencyKey:      "create-audit-alias",
		Actor:               "private-test",
		Zones: []workflow.ZoneInput{{
			ID: "zone-audit", Name: "大厅", UsageClass: "公共区域",
			AreaSquareMeters: 100, MinimumPointCount: 1, IntelligibilityThreshold: 0.6,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	events := service.Audit(caseSnapshot.ID)
	if len(events) != 1 || len(events[0].Details) == 0 {
		t.Fatalf("预期得到带详情的审计事件，实际为 %#v", events)
	}
	// 查询结果属于调用方；修改它不应改变 Store 内部摘要链。
	events[0].Details[0] ^= 1
	if err := s.VerifyAudit(); err != nil {
		t.Fatalf("修改查询结果不应破坏审计摘要链: %v", err)
	}
}
