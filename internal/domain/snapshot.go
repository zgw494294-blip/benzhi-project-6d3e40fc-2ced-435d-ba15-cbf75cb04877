package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

type CredentialSnapshot struct {
	CaseID      string             `json:"caseID"`
	CaseNumber  string             `json:"caseNumber"`
	SiteName    string             `json:"siteName"`
	Version     int64              `json:"version"`
	PlanDigest  string             `json:"planDigest"`
	RuleVersion string             `json:"ruleVersion"`
	Zones       []BroadcastZone    `json:"zones"`
	Points      []MeasurementPoint `json:"points"`
	Rounds      []MeasurementRound `json:"rounds"`
	Deviations  []Deviation        `json:"deviations"`
	Decision    *CaseDecision      `json:"decision"`
}

func StableSnapshot(c *AcceptanceCase) CredentialSnapshot {
	s := CredentialSnapshot{c.ID, c.CaseNumber, c.SiteName, c.Version, c.PlanDigest, c.RuleVersion, append([]BroadcastZone(nil), c.Zones...), append([]MeasurementPoint(nil), c.Points...), append([]MeasurementRound(nil), c.Rounds...), append([]Deviation(nil), c.Deviations...), c.Decision}
	sort.Slice(s.Zones, func(i, j int) bool { return s.Zones[i].ID < s.Zones[j].ID })
	sort.Slice(s.Points, func(i, j int) bool { return s.Points[i].ID < s.Points[j].ID })
	sort.Slice(s.Rounds, func(i, j int) bool { return s.Rounds[i].Number < s.Rounds[j].Number })
	for i := range s.Rounds {
		sort.Strings(s.Rounds[i].TargetPointIDs)
		sort.Slice(s.Rounds[i].Readings, func(a, b int) bool { return s.Rounds[i].Readings[a].PointID < s.Rounds[i].Readings[b].PointID })
	}
	sort.Slice(s.Deviations, func(i, j int) bool { return s.Deviations[i].ID < s.Deviations[j].ID })
	return s
}

func SnapshotDigest(c *AcceptanceCase) (string, error) {
	b, err := json.Marshal(StableSnapshot(c))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func SnapshotDigestForCredential(c *AcceptanceCase) (string, error) {
	if c.Credential == nil {
		return SnapshotDigest(c)
	}
	copyCase := *c
	copyCase.Version = c.Credential.CaseVersion - 1
	return SnapshotDigest(&copyCase)
}
