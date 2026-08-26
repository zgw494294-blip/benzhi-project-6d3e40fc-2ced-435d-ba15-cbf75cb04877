package domain

import (
	"errors"
	"testing"
	"time"
)

func TestDraftPlanBecomesImmutableAfterFreeze(t *testing.T) {
	now := time.Now().UTC()
	c := &AcceptanceCase{ID: "c1", CaseNumber: "A-1", SiteName: "测试建筑", ResponsibleEngineer: "工程师", Status: StatusDraft, Version: 1, CreatedAt: now, UpdatedAt: now, Zones: []BroadcastZone{{ID: "z1", CaseID: "c1", Name: "大厅", UsageClass: "公共区", AreaSquareMeters: 100, MinimumPointCount: 1, IntelligibilityThreshold: .6}}}
	points := []MeasurementPoint{{ID: "p1", ZoneID: "z1", PointCode: "P1", LocationDescription: "中央", CoverageTag: "center"}}
	if err := c.ReplacePlan(points, now); err != nil {
		t.Fatal(err)
	}
	round := MeasurementRound{ID: "r1", CaseID: c.ID, Number: 1, Kind: "initial", Status: RoundOpen, TargetPointIDs: []string{"p1"}, CreatedAt: now}
	if err := c.FreezePlan("digest", round, now); err != nil {
		t.Fatal(err)
	}
	if err := c.ReplacePlan(points, now); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("冻结后修改应失败，得到 %v", err)
	}
}

func TestCredentialSnapshotDigestIgnoresEnvelope(t *testing.T) {
	c := &AcceptanceCase{ID: "c", CaseNumber: "N", SiteName: "站点", Version: 5, RuleVersion: "r", PlanDigest: "p", Zones: []BroadcastZone{}, Points: []MeasurementPoint{}, Rounds: []MeasurementRound{}, Deviations: []Deviation{}}
	digest, err := SnapshotDigest(c)
	if err != nil {
		t.Fatal(err)
	}
	c.Version = 6
	c.Status = StatusApproved
	c.Credential = &AcceptanceCredential{CaseVersion: 6, SnapshotDigest: digest}
	recalculated, err := SnapshotDigestForCredential(c)
	if err != nil {
		t.Fatal(err)
	}
	if recalculated != digest {
		t.Fatalf("封存后摘要变化: %s != %s", recalculated, digest)
	}
}
