package contextcancelreading

import (
	"context"
	"errors"
	"testing"
	"time"

	"voice-clarity-acceptance/internal/domain"
	"voice-clarity-acceptance/internal/store"
	"voice-clarity-acceptance/internal/workflow"
)

func TestCanceledReadingRequestDoesNotPersist(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(repository)

	acceptance, err := service.CreateCase(workflow.CreateCaseCommand{
		CaseNumber:          "CTX-CASE-1",
		SiteName:            "取消测试场地",
		ResponsibleEngineer: "测试工程师",
		IdempotencyKey:      "ctx-create",
		Actor:               "测试",
		Zones: []workflow.ZoneInput{{
			ID: "ctx-zone", Name: "大厅", UsageClass: "公共大厅",
			AreaSquareMeters: 100, MinimumPointCount: 1, IntelligibilityThreshold: 0.6,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	acceptance, err = service.SavePlan(workflow.SavePlanCommand{
		CaseID: acceptance.ID, ExpectedVersion: acceptance.Version, IdempotencyKey: "ctx-plan", Actor: "测试",
		Points: []workflow.PointInput{{ID: "ctx-point", ZoneID: "ctx-zone", PointCode: "P-01", LocationDescription: "大厅中央", CoverageTag: "center"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	acceptance, err = service.FreezePlan(workflow.FreezePlanCommand{
		CaseID: acceptance.ID, ExpectedVersion: acceptance.Version, IdempotencyKey: "ctx-freeze", Actor: "测试",
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = service.AddReading(workflow.AddReadingCommand{
		CaseID: acceptance.ID, RoundID: acceptance.Rounds[0].ID, PointID: "ctx-point",
		ExpectedVersion: acceptance.Version, IdempotencyKey: "ctx-reading", Actor: "测试",
		BackgroundNoiseDBA: 45, BroadcastLevelDBA: 70, IntelligibilityValue: 0.75,
		InstrumentID: "STIPA-CTX", MeasuredAt: time.Now().UTC(), Context: ctx,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("已取消请求应返回 context.Canceled，实际错误: %v", err)
	}

	stored, err := service.GetCase(acceptance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != domain.StatusPlanFrozen || len(stored.Rounds[0].Readings) != 0 {
		t.Fatalf("取消请求不应改变案卷状态: status=%s readings=%d", stored.Status, len(stored.Rounds[0].Readings))
	}
}
