package measurement

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"voice-clarity-acceptance/internal/domain"
)

type PlanIssue struct {
	ZoneID  string `json:"zoneID,omitempty"`
	PointID string `json:"pointID,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PlanPrecheck struct {
	Revision         int            `json:"revision"`
	Version          int64          `json:"version"`
	ZoneCount        int            `json:"zoneCount"`
	PointCount       int            `json:"pointCount"`
	Coverage         map[string]int `json:"coverage"`
	CandidateSummary string         `json:"candidateSummary"`
	Issues           []PlanIssue    `json:"issues"`
	Freezable        bool           `json:"freezable"`
}

func Precheck(c *domain.AcceptanceCase) (PlanPrecheck, error) {
	issues := ValidatePlan(c)
	for _, p := range c.Points {
		if strings.TrimSpace(p.LocationDescription) == "" {
			issues = append(issues, PlanIssue{ZoneID: p.ZoneID, PointID: p.ID, Code: "empty_location", Message: "测点位置不能为空"})
		}
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].ZoneID != issues[j].ZoneID {
			return issues[i].ZoneID < issues[j].ZoneID
		}
		if issues[i].PointID != issues[j].PointID {
			return issues[i].PointID < issues[j].PointID
		}
		return issues[i].Code < issues[j].Code
	})
	coverage := map[string]int{}
	for _, p := range c.Points {
		coverage[p.ZoneID]++
	}
	for _, z := range c.Zones {
		if _, ok := coverage[z.ID]; !ok {
			coverage[z.ID] = 0
		}
	}
	ids := make([]string, 0, len(c.Points))
	for _, p := range c.Points {
		ids = append(ids, p.ID)
	}
	sort.Strings(ids)
	return PlanPrecheck{Revision: c.PlanRevision, Version: c.Version, ZoneCount: len(c.Zones), PointCount: len(c.Points), Coverage: coverage, CandidateSummary: strings.Join(ids, ","), Issues: issues, Freezable: len(issues) == 0}, nil
}

func ValidatePlan(c *domain.AcceptanceCase) []PlanIssue {
	issues := []PlanIssue{}
	byZone := map[string][]domain.MeasurementPoint{}
	codes, tags := map[string]bool{}, map[string]map[string]bool{}
	for _, p := range c.Points {
		if _, ok := c.FindZone(p.ZoneID); !ok {
			issues = append(issues, PlanIssue{ZoneID: p.ZoneID, PointID: p.ID, Code: "unknown_zone", Message: "测点所属区域不存在"})
		}
		byZone[p.ZoneID] = append(byZone[p.ZoneID], p)
		if codes[p.PointCode] {
			issues = append(issues, PlanIssue{p.ZoneID, p.ID, "duplicate_code", "测点编号重复"})
		}
		codes[p.PointCode] = true
		if tags[p.ZoneID] == nil {
			tags[p.ZoneID] = map[string]bool{}
		}
		tag := strings.TrimSpace(strings.ToLower(p.CoverageTag))
		if tags[p.ZoneID][tag] {
			issues = append(issues, PlanIssue{p.ZoneID, p.ID, "duplicate_coverage", "同一区域覆盖位置标签重复"})
		}
		if tag == "" {
			issues = append(issues, PlanIssue{ZoneID: p.ZoneID, PointID: p.ID, Code: "empty_coverage", Message: "覆盖位置标签不能为空"})
		}
		tags[p.ZoneID][tag] = true
	}
	for _, z := range c.Zones {
		if z.IntelligibilityThreshold <= 0 {
			issues = append(issues, PlanIssue{ZoneID: z.ID, Code: "missing_threshold", Message: "区域阈值不完整"})
		}
		if len(byZone[z.ID]) < z.MinimumPointCount {
			issues = append(issues, PlanIssue{ZoneID: z.ID, Code: "insufficient_points", Message: fmt.Sprintf("至少需要 %d 个测点", z.MinimumPointCount)})
		}
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].ZoneID != issues[j].ZoneID {
			return issues[i].ZoneID < issues[j].ZoneID
		}
		if issues[i].PointID != issues[j].PointID {
			return issues[i].PointID < issues[j].PointID
		}
		return issues[i].Code < issues[j].Code
	})
	return issues
}

func PlanDigest(c *domain.AcceptanceCase) (string, error) {
	type plan struct {
		Revision int                       `json:"revision"`
		Zones    []domain.BroadcastZone    `json:"zones"`
		Points   []domain.MeasurementPoint `json:"points"`
	}
	zones := append([]domain.BroadcastZone(nil), c.Zones...)
	points := append([]domain.MeasurementPoint(nil), c.Points...)
	sort.Slice(zones, func(i, j int) bool { return zones[i].ID < zones[j].ID })
	sort.Slice(points, func(i, j int) bool { return points[i].ID < points[j].ID })
	b, err := json.Marshal(plan{c.PlanRevision, zones, points})
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), nil
}
