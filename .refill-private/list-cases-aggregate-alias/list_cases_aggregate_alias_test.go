package listcasesaggregatealias

import (
	"testing"

	"voice-clarity-acceptance/internal/store"
	"voice-clarity-acceptance/internal/workflow"
)

func TestListCasesNestedMutationDoesNotPersist(t *testing.T) {
	dir := t.TempDir()
	persistence, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(persistence)
	created, err := service.CreateCase(workflow.CreateCaseCommand{
		CaseNumber:          "ALIAS-001",
		SiteName:            "应急广播中心",
		ResponsibleEngineer: "张工",
		IdempotencyKey:      "create-alias-case",
		Actor:               "private-test",
		Zones: []workflow.ZoneInput{{
			ID:                       "zone-main",
			Name:                     "主大厅",
			UsageClass:               "公共大厅",
			AreaSquareMeters:         120,
			MinimumPointCount:        1,
			IntelligibilityThreshold: 0.60,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	listed, err := service.ListCases()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || len(listed[0].Zones) != 1 {
		t.Fatalf("列表结果不完整: %#v", listed)
	}
	listed[0].Zones[0].Name = "调用方污染的区域名"

	_, err = service.SavePlan(workflow.SavePlanCommand{
		CaseID:          created.ID,
		ExpectedVersion: created.Version,
		IdempotencyKey:  "save-plan-after-list",
		Actor:           "private-test",
		Points: []workflow.PointInput{{
			ID:                  "point-main",
			ZoneID:              "zone-main",
			PointCode:           "P-01",
			LocationDescription: "大厅中央",
			CoverageTag:         "center",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	restarted, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := restarted.GetCase(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Zones[0].Name != "主大厅" {
		t.Fatalf("列表查询结果的修改被后续写命令持久化: got %q", got.Zones[0].Name)
	}
}
