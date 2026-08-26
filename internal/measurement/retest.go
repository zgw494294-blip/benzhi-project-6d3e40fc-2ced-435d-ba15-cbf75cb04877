package measurement

import (
	"sort"
	"voice-clarity-acceptance/internal/domain"
)

type RetestCandidate struct {
	PointID   string  `json:"pointID"`
	ZoneID    string  `json:"zoneID"`
	Reason    string  `json:"reason"`
	Value     float64 `json:"value,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
}

func Candidates(c *domain.AcceptanceCase, zoneID string) ([]RetestCandidate, error) {
	if c.Decision == nil {
		return nil, domain.ErrStateConflict
	}
	zone, ok := c.FindZone(zoneID)
	if !ok {
		return nil, domain.ErrNotFound
	}
	result := []RetestCandidate{}
	for _, p := range c.Decision.Points {
		point, exists := c.FindPoint(p.PointID)
		if !exists || point.ZoneID != zone.ID || (p.Valid && p.Passed) {
			continue
		}
		reason := p.Reason
		if reason == "" {
			reason = "未通过"
		}
		result = append(result, RetestCandidate{PointID: p.PointID, ZoneID: zoneID, Reason: reason, Value: p.Value, Threshold: p.Threshold})
	}
	if c.Status == domain.StatusReturned && len(result) == 0 {
		seen := map[string]bool{}
		for _, review := range c.Reviews {
			for _, issue := range review.Issues {
				if issue.PointID == "" || seen[issue.PointID] {
					continue
				}
				point, ok := c.FindPoint(issue.PointID)
				if ok && point.ZoneID == zoneID {
					result = append(result, RetestCandidate{PointID: issue.PointID, ZoneID: zoneID, Reason: issue.Description})
					seen[issue.PointID] = true
				}
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].PointID < result[j].PointID })
	return result, nil
}

func RetestTargets(c *domain.AcceptanceCase, zoneID string, requested []string) ([]string, error) {
	if c.Decision == nil {
		return nil, domain.ErrStateConflict
	}
	failed := map[string]bool{}
	for _, p := range c.Decision.Points {
		if !p.Valid || !p.Passed {
			failed[p.PointID] = true
		}
	}
	allowed := map[string]bool{}
	returnedTargets := map[string]bool{}
	if c.Status == domain.StatusReturned {
		for _, review := range c.Reviews {
			for _, issue := range review.Issues {
				if issue.PointID != "" {
					returnedTargets[issue.PointID] = true
				}
			}
		}
	}
	for _, p := range c.Points {
		if p.ZoneID == zoneID && (failed[p.ID] || returnedTargets[p.ID]) {
			allowed[p.ID] = true
		}
	}
	if len(requested) == 0 {
		for id := range allowed {
			requested = append(requested, id)
		}
	}
	seen := map[string]bool{}
	targets := []string{}
	for _, id := range requested {
		if seen[id] {
			return nil, &domain.FieldError{Field: "targetPointIDs", Message: "目标测点不能重复"}
		}
		if !allowed[id] {
			return nil, &domain.FieldError{Field: "targetPointIDs", Message: "只能包含该失败区域的失败或无效测点"}
		}
		seen[id] = true
		targets = append(targets, id)
	}
	for _, r := range c.Rounds {
		if r.Status == domain.RoundOpen && r.Kind == "retest" {
			for _, id := range targets {
				for _, occupied := range r.TargetPointIDs {
					if id == occupied {
						return nil, &domain.FieldError{Field: "targetPointIDs", Message: "测点已被开放复测轮次占用"}
					}
				}
			}
		}
	}
	if len(targets) == 0 {
		return nil, &domain.FieldError{Field: "targetPointIDs", Message: "没有可复测的失败测点"}
	}
	sort.Strings(targets)
	return targets, nil
}
