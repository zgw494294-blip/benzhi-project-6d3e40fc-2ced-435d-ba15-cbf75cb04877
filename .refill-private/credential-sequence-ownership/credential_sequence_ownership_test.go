package credentialsequenceownership

import (
	"testing"
	"time"

	"voice-clarity-acceptance/internal/domain"
	"voice-clarity-acceptance/internal/store"
	"voice-clarity-acceptance/internal/workflow"
)

func readyForApproval(t *testing.T, service *workflow.Service, suffix string) *domain.AcceptanceCase {
	t.Helper()
	c, err := service.CreateCase(workflow.CreateCaseCommand{
		CaseNumber:          "SEQ-" + suffix,
		SiteName:            "凭据序号并发验收点",
		ResponsibleEngineer: "测试工程师",
		IdempotencyKey:      "create-" + suffix,
		Actor:               "测试工程师",
		Zones: []workflow.ZoneInput{{
			ID: "zone-" + suffix, Name: "大厅", UsageClass: "公共大厅",
			AreaSquareMeters: 80, MinimumPointCount: 1, IntelligibilityThreshold: 0.6,
		}},
	})
	if err != nil {
		t.Fatalf("创建验收案失败: %v", err)
	}
	c, err = service.SavePlan(workflow.SavePlanCommand{
		CaseID: c.ID, ExpectedVersion: c.Version, IdempotencyKey: "plan-" + suffix, Actor: "测试工程师",
		Points: []workflow.PointInput{{
			ID: "point-" + suffix, ZoneID: "zone-" + suffix, PointCode: "P-" + suffix,
			LocationDescription: "大厅中央", CoverageTag: "center-" + suffix,
		}},
	})
	if err != nil {
		t.Fatalf("保存测点计划失败: %v", err)
	}
	c, err = service.FreezePlan(workflow.FreezePlanCommand{
		CaseID: c.ID, ExpectedVersion: c.Version, IdempotencyKey: "freeze-" + suffix, Actor: "测试工程师",
	})
	if err != nil {
		t.Fatalf("冻结测点计划失败: %v", err)
	}
	roundID := c.Rounds[0].ID
	c, err = service.AddReading(workflow.AddReadingCommand{
		CaseID: c.ID, RoundID: roundID, PointID: "point-" + suffix,
		ExpectedVersion: c.Version, IdempotencyKey: "reading-" + suffix, Actor: "测试工程师",
		BackgroundNoiseDBA: 42, BroadcastLevelDBA: 70, IntelligibilityValue: 0.75,
		InstrumentID: "STIPA-" + suffix, MeasuredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("录入读数失败: %v", err)
	}
	c, err = service.CloseRound(workflow.CloseRoundCommand{
		CaseID: c.ID, RoundID: roundID, ExpectedVersion: c.Version,
		IdempotencyKey: "close-" + suffix, Actor: "测试工程师",
	})
	if err != nil {
		t.Fatalf("关闭测量轮次失败: %v", err)
	}
	if c.Status != domain.StatusPendingReview {
		t.Fatalf("验收案未进入待审核状态: %s", c.Status)
	}
	return c
}

func TestConcurrentServicesAllocateDistinctCredentialSequences(t *testing.T) {
	dir := t.TempDir()
	persistence, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	firstService := workflow.New(persistence)
	secondService := workflow.New(persistence)
	firstCase := readyForApproval(t, firstService, "A")
	secondCase := readyForApproval(t, secondService, "B")

	type result struct {
		caseValue *domain.AcceptanceCase
		err       error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	approve := func(service *workflow.Service, c *domain.AcceptanceCase, key, reviewer string) {
		<-start
		approved, reviewErr := service.Review(workflow.ReviewCommand{
			CaseID: c.ID, Decision: "approve", Reviewer: reviewer,
			ExpectedVersion: c.Version, IdempotencyKey: key,
		})
		results <- result{caseValue: approved, err: reviewErr}
	}
	go approve(firstService, firstCase, "approve-A", "审核员甲")
	go approve(secondService, secondCase, "approve-B", "审核员乙")
	close(start)

	firstResult := <-results
	secondResult := <-results
	if firstResult.err != nil || secondResult.err != nil {
		t.Fatalf("并发批准不应失败: first=%v second=%v", firstResult.err, secondResult.err)
	}
	if firstResult.caseValue.Credential.Sequence == secondResult.caseValue.Credential.Sequence {
		t.Errorf("两个成功批准的案卷获得了重复凭据序号: %d", firstResult.caseValue.Credential.Sequence)
	}
	if _, err = store.Open(dir); err != nil {
		t.Fatalf("成功批准后应能从快照和审计日志恢复: %v", err)
	}
}
