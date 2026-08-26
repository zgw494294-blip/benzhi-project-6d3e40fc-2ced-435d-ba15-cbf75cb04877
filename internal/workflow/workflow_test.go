package workflow

import (
	"errors"
	"testing"
	"time"
	"voice-clarity-acceptance/internal/domain"
	"voice-clarity-acceptance/internal/store"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(s)
}

func preparedCase(t *testing.T, service *Service) *domain.AcceptanceCase {
	t.Helper()
	c, err := service.CreateCase(CreateCaseCommand{CaseNumber: "CASE-1", SiteName: "测试中心", ResponsibleEngineer: "张工", IdempotencyKey: "create", Actor: "tester", Zones: []ZoneInput{{ID: "z1", Name: "大厅", UsageClass: "公共大厅", AreaSquareMeters: 100, MinimumPointCount: 2, IntelligibilityThreshold: .6}}})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.SavePlan(SavePlanCommand{CaseID: c.ID, ExpectedVersion: c.Version, IdempotencyKey: "plan", Actor: "tester", Points: []PointInput{{ID: "p1", ZoneID: "z1", PointCode: "P1", LocationDescription: "北侧", CoverageTag: "north"}, {ID: "p2", ZoneID: "z1", PointCode: "P2", LocationDescription: "南侧", CoverageTag: "south"}}})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.FreezePlan(FreezePlanCommand{CaseID: c.ID, ExpectedVersion: c.Version, IdempotencyKey: "freeze", Actor: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func addReading(t *testing.T, service *Service, c *domain.AcceptanceCase, roundID, pointID, key string, value float64) *domain.AcceptanceCase {
	t.Helper()
	next, err := service.AddReading(AddReadingCommand{CaseID: c.ID, RoundID: roundID, PointID: pointID, ExpectedVersion: c.Version, IdempotencyKey: key, Actor: "tester", BackgroundNoiseDBA: 45, BroadcastLevelDBA: 70, IntelligibilityValue: value, InstrumentID: "STIPA-1", MeasuredAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func TestFailureDeviationRetestApprovalAndCredential(t *testing.T) {
	service := newTestService(t)
	c := preparedCase(t, service)
	roundID := c.Rounds[0].ID
	c = addReading(t, service, c, roundID, "p1", "read-1", .72)
	c = addReading(t, service, c, roundID, "p2", "read-2", .42)
	var err error
	c, err = service.CloseRound(CloseRoundCommand{CaseID: c.ID, RoundID: roundID, ExpectedVersion: c.Version, IdempotencyKey: "close-1", Actor: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != domain.StatusRemediation || c.Decision.Passed {
		t.Fatalf("初测失败应进入整改: %s", c.Status)
	}
	if _, err = service.Review(ReviewCommand{CaseID: c.ID, ExpectedVersion: c.Version, IdempotencyKey: "early-review", Decision: "approve", Reviewer: "审核员"}); err == nil {
		t.Fatalf("整改中批准应被门禁阻止: %v", err)
	}
	c, err = service.CreateDeviation(CreateDeviationCommand{CaseID: c.ID, ZoneID: "z1", Reason: "声场遮挡", CorrectiveAction: "调整扬声器", TargetPointIDs: []string{"p2"}, ExpectedVersion: c.Version, IdempotencyKey: "deviation", Actor: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	retest := c.Rounds[1].ID
	c = addReading(t, service, c, retest, "p2", "read-3", .75)
	c, err = service.CloseRound(CloseRoundCommand{CaseID: c.ID, RoundID: retest, ExpectedVersion: c.Version, IdempotencyKey: "close-2", Actor: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != domain.StatusPendingReview || c.Deviations[0].Status != domain.DeviationClosed {
		t.Fatalf("复测通过应关闭偏差并待审核: %s %#v", c.Status, c.Deviations)
	}
	c, err = service.Review(ReviewCommand{CaseID: c.ID, ExpectedVersion: c.Version, IdempotencyKey: "approve", Decision: "approve", Reviewer: "李审核", Comment: "证据完整"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != domain.StatusApproved || c.Credential == nil || len(c.Rounds[0].Readings) != 2 {
		t.Fatalf("批准或历史证据异常: %#v", c)
	}
	verification, err := service.VerifyCredential(c.ID)
	if err != nil || !verification.Valid {
		t.Fatalf("凭据校验失败: %#v %v", verification, err)
	}
}

func TestVersionConflictAndIdempotentReplay(t *testing.T) {
	service := newTestService(t)
	c := preparedCase(t, service)
	round := c.Rounds[0].ID
	after := addReading(t, service, c, round, "p1", "same-key", .7)
	replayed, err := service.AddReading(AddReadingCommand{CaseID: c.ID, RoundID: round, PointID: "p1", ExpectedVersion: c.Version, IdempotencyKey: "same-key"})
	if err != nil || replayed.Version != after.Version {
		t.Fatalf("幂等重放未复用结果: %v %#v", err, replayed)
	}
	_, err = service.AddReading(AddReadingCommand{CaseID: c.ID, RoundID: round, PointID: "p2", ExpectedVersion: c.Version, IdempotencyKey: "stale-key", BackgroundNoiseDBA: 45, BroadcastLevelDBA: 70, IntelligibilityValue: .7, InstrumentID: "I", MeasuredAt: time.Now().UTC()})
	if !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("过期版本应冲突: %v", err)
	}
}
