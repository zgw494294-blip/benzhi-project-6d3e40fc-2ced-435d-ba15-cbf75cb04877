package batchreplaymetadata

import (
	"testing"
	"time"
	"voice-clarity-acceptance/internal/store"
	"voice-clarity-acceptance/internal/workflow"
)

func TestBatchReadingReplayPreservesResultMetadata(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := workflow.New(st)
	c, err := s.CreateCase(workflow.CreateCaseCommand{CaseNumber: "BATCH-1", SiteName: "批量测试", ResponsibleEngineer: "工程师", IdempotencyKey: "create", Zones: []workflow.ZoneInput{{ID: "z", Name: "大厅", UsageClass: "大厅", AreaSquareMeters: 100, MinimumPointCount: 2, IntelligibilityThreshold: .6}}})
	if err != nil {
		t.Fatal(err)
	}
	c, err = s.SavePlan(workflow.SavePlanCommand{CaseID: c.ID, ExpectedVersion: c.Version, IdempotencyKey: "plan", Points: []workflow.PointInput{{ID: "p1", ZoneID: "z", PointCode: "P1", LocationDescription: "北", CoverageTag: "north"}, {ID: "p2", ZoneID: "z", PointCode: "P2", LocationDescription: "南", CoverageTag: "south"}}})
	if err != nil {
		t.Fatal(err)
	}
	c, err = s.FreezePlan(workflow.FreezePlanCommand{CaseID: c.ID, ExpectedVersion: c.Version, IdempotencyKey: "freeze"})
	if err != nil {
		t.Fatal(err)
	}
	entries := []workflow.ReadingEntry{{PointID: "p1", BackgroundNoiseDBA: 40, BroadcastLevelDBA: 70, IntelligibilityValue: .8, InstrumentID: "I", MeasuredAt: time.Now().UTC()}, {PointID: "p2", BackgroundNoiseDBA: 40, BroadcastLevelDBA: 70, IntelligibilityValue: .8, InstrumentID: "I", MeasuredAt: time.Now().UTC()}}
	first, err := s.AddReadings(workflow.AddReadingsCommand{CaseID: c.ID, RoundID: c.Rounds[0].ID, ExpectedVersion: c.Version, IdempotencyKey: "batch", Entries: entries})
	if err != nil {
		t.Fatal(err)
	}
	if first.Submitted != 2 || first.Remaining != 0 {
		t.Fatalf("首次批量提交结果异常: %+v", first)
	}
	replay, err := s.AddReadings(workflow.AddReadingsCommand{CaseID: c.ID, RoundID: c.Rounds[0].ID, ExpectedVersion: c.Version, IdempotencyKey: "batch", Entries: entries})
	if err != nil {
		t.Fatal(err)
	}
	if replay.Submitted != first.Submitted || replay.Invalid != first.Invalid || replay.Remaining != first.Remaining {
		t.Fatalf("幂等重放丢失批量结果元数据: first=%+v replay=%+v", first, replay)
	}
}
