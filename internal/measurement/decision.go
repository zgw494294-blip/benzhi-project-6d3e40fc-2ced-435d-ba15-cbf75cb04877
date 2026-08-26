package measurement

import (
	"sort"
	"time"
	"voice-clarity-acceptance/internal/domain"
)

func Calculate(c *domain.AcceptanceCase, now time.Time) domain.CaseDecision {
	latest := latestReadings(c)
	result := domain.CaseDecision{RuleVersion: RuleVersion, Passed: true, CalculatedAt: now, Points: []domain.PointDecision{}, Zones: []domain.ZoneDecision{}, FailedZoneIDs: []string{}, MissingPointIDs: []string{}, InvalidPointIDs: []string{}}
	for _, zone := range c.Zones {
		zd := domain.ZoneDecision{ZoneID: zone.ID, Passed: true, RequiredPoints: zone.MinimumPointCount, FailedPointIDs: []string{}, MissingPointIDs: []string{}, InvalidPointIDs: []string{}}
		for _, point := range c.Points {
			if point.ZoneID != zone.ID {
				continue
			}
			reading, ok := latest[point.ID]
			if !ok {
				zd.Passed = false
				zd.MissingPointIDs = append(zd.MissingPointIDs, point.ID)
				result.MissingPointIDs = append(result.MissingPointIDs, point.ID)
				result.Points = append(result.Points, domain.PointDecision{PointID: point.ID, Threshold: zone.IntelligibilityThreshold, Reason: "缺少有效轮次读数", SourceKind: "missing"})
				continue
			}
			kind := "initial"
			if r, ok := c.FindRound(reading.RoundID); ok && r.Kind == "retest" {
				kind = "retest"
			}
			pd := domain.PointDecision{PointID: point.ID, RoundID: reading.RoundID, Valid: reading.ValidityStatus == domain.ReadingValid, Value: reading.IntelligibilityValue, Threshold: zone.IntelligibilityThreshold, Difference: reading.IntelligibilityValue - zone.IntelligibilityThreshold, MeasuredAt: reading.MeasuredAt, SourceKind: kind}
			if !pd.Valid {
				pd.Reason = reading.InvalidReason
				pd.ValidityReason = reading.InvalidReason
				zd.InvalidPointIDs = append(zd.InvalidPointIDs, point.ID)
				result.InvalidPointIDs = append(result.InvalidPointIDs, point.ID)
				zd.Passed = false
			} else {
				zd.MeasuredPoints++
				pd.Passed = reading.IntelligibilityValue >= zone.IntelligibilityThreshold
				if !pd.Passed {
					pd.Reason = "可懂度低于冻结阈值"
					zd.FailedPointIDs = append(zd.FailedPointIDs, point.ID)
					zd.Passed = false
				}
			}
			result.Points = append(result.Points, pd)
		}
		zd.MissingCount = len(zd.MissingPointIDs)
		zd.InvalidCount = len(zd.InvalidPointIDs)
		zd.FailedCount = len(zd.FailedPointIDs)
		if zd.MeasuredPoints < zd.RequiredPoints {
			zd.Passed = false
		}
		if !zd.Passed {
			result.Passed = false
			result.FailedZoneIDs = append(result.FailedZoneIDs, zone.ID)
		}
		result.Zones = append(result.Zones, zd)
	}
	sort.Slice(result.Points, func(i, j int) bool { return result.Points[i].PointID < result.Points[j].PointID })
	sort.Slice(result.Zones, func(i, j int) bool { return result.Zones[i].ZoneID < result.Zones[j].ZoneID })
	sort.Strings(result.FailedZoneIDs)
	sort.Strings(result.MissingPointIDs)
	sort.Strings(result.InvalidPointIDs)
	return result
}

func latestReadings(c *domain.AcceptanceCase) map[string]domain.MeasurementReading {
	result := map[string]domain.MeasurementReading{}
	rounds := append([]domain.MeasurementRound(nil), c.Rounds...)
	sort.Slice(rounds, func(i, j int) bool { return rounds[i].Number < rounds[j].Number })
	for _, r := range rounds {
		for _, reading := range r.Readings {
			result[reading.PointID] = reading
		}
	}
	return result
}

func RoundComplete(c *domain.AcceptanceCase, round *domain.MeasurementRound) (bool, []string) {
	read := map[string]bool{}
	for _, r := range round.Readings {
		read[r.PointID] = true
	}
	missing := []string{}
	for _, id := range round.TargetPointIDs {
		if !read[id] {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	return len(missing) == 0, missing
}
