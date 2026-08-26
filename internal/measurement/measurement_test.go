package measurement

import (
	"testing"
	"time"
	"voice-clarity-acceptance/internal/domain"
)

func TestPlanCoverageAndDuplicateTags(t *testing.T) {
	c := &domain.AcceptanceCase{Zones: []domain.BroadcastZone{{ID: "z", MinimumPointCount: 2, IntelligibilityThreshold: .6}}, Points: []domain.MeasurementPoint{{ID: "p1", ZoneID: "z", PointCode: "1", CoverageTag: "north"}, {ID: "p2", ZoneID: "z", PointCode: "2", CoverageTag: "NORTH"}}}
	issues := ValidatePlan(c)
	if len(issues) != 1 || issues[0].Code != "duplicate_coverage" {
		t.Fatalf("预期覆盖标签重复，得到 %#v", issues)
	}
}

func TestReadingHardBoundsAndQualityInvalidity(t *testing.T) {
	now := time.Now().UTC()
	bad := ReadingInput{BackgroundNoiseDBA: 5, BroadcastLevelDBA: 70, IntelligibilityValue: .7, InstrumentID: "I", MeasuredAt: now}
	if err := ValidateReadingFields(bad, now); err == nil {
		t.Fatal("越界背景噪声应被拒绝")
	}
	quality := ReadingInput{BackgroundNoiseDBA: 60, BroadcastLevelDBA: 62, IntelligibilityValue: .8, InstrumentID: "I", MeasuredAt: now}
	status, reason := ValidateReading(quality, now)
	if status != domain.ReadingInvalid || reason == "" {
		t.Fatalf("低信噪比应保存为有明确原因的无效读数: %s %s", status, reason)
	}
}

func TestLatestRetestReadingOverridesOriginalFailure(t *testing.T) {
	now := time.Now().UTC()
	c := &domain.AcceptanceCase{Zones: []domain.BroadcastZone{{ID: "z", MinimumPointCount: 1, IntelligibilityThreshold: .6}}, Points: []domain.MeasurementPoint{{ID: "p", ZoneID: "z"}}, Rounds: []domain.MeasurementRound{
		{ID: "r1", Number: 1, Readings: []domain.MeasurementReading{{RoundID: "r1", PointID: "p", IntelligibilityValue: .4, ValidityStatus: domain.ReadingValid}}},
		{ID: "r2", Number: 2, Readings: []domain.MeasurementReading{{RoundID: "r2", PointID: "p", IntelligibilityValue: .8, ValidityStatus: domain.ReadingValid}}},
	}}
	decision := Calculate(c, now)
	if !decision.Passed || decision.Points[0].RoundID != "r2" {
		t.Fatalf("最新复测应覆盖判定但保留原轮证据: %#v", decision)
	}
}
