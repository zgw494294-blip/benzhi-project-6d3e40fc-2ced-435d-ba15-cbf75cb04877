package measurement

import (
	"sort"
	"strings"
	"time"
	"voice-clarity-acceptance/internal/domain"
)

type PointQuality struct {
	ZoneID  string `json:"zoneID"`
	PointID string `json:"pointID"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
}

type QualityGate struct {
	RoundID   string         `json:"roundID"`
	Target    int            `json:"target"`
	Recorded  int            `json:"recorded"`
	Missing   int            `json:"missing"`
	Valid     int            `json:"valid"`
	Invalid   int            `json:"invalid"`
	Conflicts int            `json:"conflicts"`
	Items     []PointQuality `json:"items"`
	CanClose  bool           `json:"canClose"`
}

func Gate(c *domain.AcceptanceCase, roundID string, now time.Time) (QualityGate, error) {
	r, ok := c.FindRound(roundID)
	if !ok {
		return QualityGate{}, domain.ErrNotFound
	}
	items := make([]PointQuality, 0, len(r.TargetPointIDs))
	missing, invalid, valid, conflicts := 0, 0, 0, 0
	for _, id := range r.TargetPointIDs {
		p, exists := c.FindPoint(id)
		if !exists {
			conflicts++
			items = append(items, PointQuality{PointID: id, Status: "conflict", Reason: "测点不属于当前计划"})
			continue
		}
		readings := []domain.MeasurementReading{}
		for _, x := range r.Readings {
			if x.PointID == id {
				readings = append(readings, x)
			}
		}
		if len(readings) == 0 {
			missing++
			items = append(items, PointQuality{ZoneID: p.ZoneID, PointID: id, Status: "missing", Reason: "缺少读数"})
			continue
		}
		active := 0
		var chosen domain.MeasurementReading
		for _, x := range readings {
			if x.SupersededByReadingID == "" {
				active++
				chosen = x
			}
		}
		if active != 1 {
			conflicts++
			items = append(items, PointQuality{ZoneID: p.ZoneID, PointID: id, Status: "conflict", Reason: "同测点存在未正确替代的证据"})
			continue
		}
		if strings.TrimSpace(chosen.InstrumentID) == "" || chosen.MeasuredAt.IsZero() || chosen.MeasuredAt.After(now) {
			conflicts++
			items = append(items, PointQuality{ZoneID: p.ZoneID, PointID: id, Status: "conflict", Reason: "仪器标识或测量时间无效"})
			continue
		}
		if chosen.ValidityStatus == domain.ReadingInvalid {
			invalid++
			items = append(items, PointQuality{ZoneID: p.ZoneID, PointID: id, Status: "invalid", Reason: chosen.InvalidReason})
			continue
		}
		valid++
		items = append(items, PointQuality{ZoneID: p.ZoneID, PointID: id, Status: "valid"})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].ZoneID != items[j].ZoneID {
			return items[i].ZoneID < items[j].ZoneID
		}
		return items[i].PointID < items[j].PointID
	})
	unique := map[string]bool{}
	for _, x := range r.Readings {
		unique[x.PointID] = true
	}
	return QualityGate{RoundID: roundID, Target: len(r.TargetPointIDs), Recorded: len(unique), Missing: missing, Valid: valid, Invalid: invalid, Conflicts: conflicts, Items: items, CanClose: missing == 0 && conflicts == 0}, nil
}

type ReviewGrade string

const (
	GradeFail       ReviewGrade = "fail"
	GradeBorderline ReviewGrade = "borderline_pass"
	GradeStable     ReviewGrade = "stable_pass"
	GradeInvalid    ReviewGrade = "invalid"
	GradeMissing    ReviewGrade = "missing"
)

type ReviewPoint struct {
	ZoneID       string    `json:"zoneID"`
	PointID      string    `json:"pointID"`
	RoundID      string    `json:"roundID,omitempty"`
	InstrumentID string    `json:"instrumentID,omitempty"`
	Status       string    `json:"status"`
	Reason       string    `json:"reason,omitempty"`
	Difference   float64   `json:"difference"`
	Threshold    float64   `json:"threshold"`
	Value        float64   `json:"value,omitempty"`
	MeasuredAt   time.Time `json:"measuredAt,omitempty"`
}
type ZoneGrade struct {
	ZoneID            string              `json:"zoneID"`
	Counts            map[ReviewGrade]int `json:"counts"`
	MinPassDifference float64             `json:"minPassDifference"`
	MaxFailDifference float64             `json:"maxFailDifference"`
}
type ReviewGrades struct {
	RuleVersion        string        `json:"ruleVersion"`
	CaseVersion        int64         `json:"caseVersion"`
	BorderlineBoundary float64       `json:"borderlineBoundary"`
	Zones              []ZoneGrade   `json:"zones"`
	Items              []ReviewPoint `json:"items"`
}

func Grade(c *domain.AcceptanceCase, zoneFilter, gradeFilter, roundFilter string) (ReviewGrades, error) {
	if zoneFilter != "" {
		if _, ok := c.FindZone(zoneFilter); !ok {
			return ReviewGrades{}, &domain.FieldError{Field: "zoneID", Message: "区域不属于当前验收案"}
		}
	}
	if roundFilter != "" {
		if _, ok := c.FindRound(roundFilter); !ok {
			return ReviewGrades{}, &domain.FieldError{Field: "roundID", Message: "轮次不属于当前验收案"}
		}
	}
	allowedGrades := map[string]bool{string(GradeFail): true, string(GradeBorderline): true, string(GradeStable): true, string(GradeInvalid): true, string(GradeMissing): true}
	if gradeFilter != "" && !allowedGrades[gradeFilter] {
		return ReviewGrades{}, &domain.FieldError{Field: "grade", Message: "未知复核分级"}
	}
	latest := map[string]domain.MeasurementReading{}
	rounds := append([]domain.MeasurementRound(nil), c.Rounds...)
	sort.Slice(rounds, func(i, j int) bool { return rounds[i].Number < rounds[j].Number })
	for _, r := range rounds {
		for _, x := range r.Readings {
			if x.SupersededByReadingID == "" {
				latest[x.PointID] = x
			}
		}
	}
	rule := c.RuleVersion
	if rule == "" {
		rule = RuleVersion
	}
	result := ReviewGrades{RuleVersion: rule, CaseVersion: c.Version, BorderlineBoundary: 0.02, Zones: []ZoneGrade{}, Items: []ReviewPoint{}}
	for _, p := range c.Points {
		if zoneFilter != "" && p.ZoneID != zoneFilter {
			continue
		}
		z, _ := c.FindZone(p.ZoneID)
		x, ok := latest[p.ID]
		item := ReviewPoint{ZoneID: p.ZoneID, PointID: p.ID, Threshold: z.IntelligibilityThreshold}
		if !ok {
			item.Status = string(GradeMissing)
			item.Reason = "缺少有效读数"
		} else {
			item.RoundID = x.RoundID
			item.Value = x.IntelligibilityValue
			item.Difference = x.IntelligibilityValue - z.IntelligibilityThreshold
			item.InstrumentID = x.InstrumentID
			item.MeasuredAt = x.MeasuredAt
			if x.ValidityStatus == domain.ReadingInvalid {
				item.Status = string(GradeInvalid)
				item.Reason = x.InvalidReason
			} else if item.Difference < 0 {
				item.Status = string(GradeFail)
				item.Reason = "低于冻结阈值"
			} else if item.Difference <= result.BorderlineBoundary {
				item.Status = string(GradeBorderline)
			} else {
				item.Status = string(GradeStable)
			}
		}
		if roundFilter != "" && item.RoundID != roundFilter {
			continue
		}
		if gradeFilter != "" && item.Status != gradeFilter {
			continue
		}
		result.Items = append(result.Items, item)
	}
	for _, z := range c.Zones {
		zg := ZoneGrade{ZoneID: z.ID, Counts: map[ReviewGrade]int{}}
		minSet, maxSet := false, false
		for _, x := range result.Items {
			if x.ZoneID != z.ID {
				continue
			}
			g := ReviewGrade(x.Status)
			zg.Counts[g]++
			if g == GradeBorderline || g == GradeStable {
				if !minSet || x.Difference < zg.MinPassDifference {
					zg.MinPassDifference = x.Difference
					minSet = true
				}
			}
			if g == GradeFail {
				if !maxSet || x.Difference > zg.MaxFailDifference {
					zg.MaxFailDifference = x.Difference
					maxSet = true
				}
			}
		}
		if len(zg.Counts) > 0 {
			result.Zones = append(result.Zones, zg)
		}
	}
	sort.Slice(result.Items, func(i, j int) bool {
		if result.Items[i].ZoneID != result.Items[j].ZoneID {
			return result.Items[i].ZoneID < result.Items[j].ZoneID
		}
		return result.Items[i].PointID < result.Items[j].PointID
	})
	return result, nil
}
