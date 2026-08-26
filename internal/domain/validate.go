package domain

import (
	"fmt"
	"strings"
)

func ValidateNewCase(c *AcceptanceCase) error {
	if strings.TrimSpace(c.CaseNumber) == "" {
		return &FieldError{"caseNumber", "不能为空"}
	}
	if strings.TrimSpace(c.SiteName) == "" {
		return &FieldError{"siteName", "不能为空"}
	}
	if strings.TrimSpace(c.ResponsibleEngineer) == "" {
		return &FieldError{"responsibleEngineer", "不能为空"}
	}
	if len(c.Zones) == 0 {
		return &FieldError{"zones", "至少登记一个广播区域"}
	}
	seen := map[string]bool{}
	seenNames := map[string]bool{}
	for i := range c.Zones {
		z := &c.Zones[i]
		if strings.TrimSpace(z.ID) == "" {
			return &FieldError{fmt.Sprintf("zones[%d].id", i), "不能为空"}
		}
		if seen[z.ID] {
			return &FieldError{"zones", "区域 ID 重复"}
		}
		seen[z.ID] = true
		if strings.TrimSpace(z.Name) == "" {
			return &FieldError{fmt.Sprintf("zones[%d].name", i), "不能为空"}
		}
		nameKey := strings.ToLower(strings.TrimSpace(z.Name))
		if seenNames[nameKey] {
			return &FieldError{fmt.Sprintf("zones[%d].name", i), "区域名称不能重复"}
		}
		seenNames[nameKey] = true
		if strings.TrimSpace(z.UsageClass) == "" {
			return &FieldError{fmt.Sprintf("zones[%d].usageClass", i), "不能为空"}
		}
		if z.AreaSquareMeters <= 0 {
			return &FieldError{fmt.Sprintf("zones[%d].areaSquareMeters", i), "必须大于 0"}
		}
		if z.MinimumPointCount < 1 {
			return &FieldError{fmt.Sprintf("zones[%d].minimumPointCount", i), "至少为 1"}
		}
		if z.IntelligibilityThreshold < 0.30 || z.IntelligibilityThreshold > 1 {
			return &FieldError{fmt.Sprintf("zones[%d].intelligibilityThreshold", i), "必须在 0.30 到 1.00 之间"}
		}
	}
	return nil
}

func ValidatePoints(c *AcceptanceCase, points []MeasurementPoint) error {
	if c.Status != StatusDraft {
		return ErrStateConflict
	}
	zones := map[string]bool{}
	for _, z := range c.Zones {
		zones[z.ID] = true
	}
	ids, codes := map[string]bool{}, map[string]bool{}
	for i, p := range points {
		if !zones[p.ZoneID] {
			return &FieldError{fmt.Sprintf("points[%d].zoneID", i), "区域不存在"}
		}
		if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.PointCode) == "" {
			return &FieldError{fmt.Sprintf("points[%d]", i), "ID 和编号不能为空"}
		}
		if ids[p.ID] || codes[p.PointCode] {
			return &FieldError{"points", "测点 ID 或编号重复"}
		}
		ids[p.ID], codes[p.PointCode] = true, true
		if strings.TrimSpace(p.LocationDescription) == "" || strings.TrimSpace(p.CoverageTag) == "" {
			return &FieldError{fmt.Sprintf("points[%d]", i), "位置和覆盖标签不能为空"}
		}
	}
	return nil
}

func (c *AcceptanceCase) EnsureMutable() error {
	if c.Status == StatusApproved || c.Credential != nil {
		return ErrImmutable
	}
	return nil
}

func (c *AcceptanceCase) CheckVersion(expected int64) error {
	if expected != c.Version {
		return &ConflictError{Expected: expected, Actual: c.Version}
	}
	return nil
}

func (c *AcceptanceCase) FindZone(id string) (*BroadcastZone, bool) {
	for i := range c.Zones {
		if c.Zones[i].ID == id {
			return &c.Zones[i], true
		}
	}
	return nil, false
}

func (c *AcceptanceCase) FindPoint(id string) (*MeasurementPoint, bool) {
	for i := range c.Points {
		if c.Points[i].ID == id {
			return &c.Points[i], true
		}
	}
	return nil, false
}

func (c *AcceptanceCase) FindRound(id string) (*MeasurementRound, bool) {
	for i := range c.Rounds {
		if c.Rounds[i].ID == id {
			return &c.Rounds[i], true
		}
	}
	return nil, false
}
