package workflow

import (
	"strings"
	"voice-clarity-acceptance/internal/domain"
)

type ZoneInput struct {
	ID, Name, UsageClass, Notes string
	AreaSquareMeters            float64
	MinimumPointCount           int
	IntelligibilityThreshold    float64
}
type CreateCaseCommand struct {
	CaseNumber, SiteName, ResponsibleEngineer, IdempotencyKey, Actor string
	ExpectedVersion                                                  int64
	Zones                                                            []ZoneInput
}

func (s *Service) CreateCase(cmd CreateCaseCommand) (*domain.AcceptanceCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok, err := s.existing(cmd.IdempotencyKey, "create_case"); ok || err != nil {
		return c, err
	}
	if cmd.ExpectedVersion != 0 {
		return nil, &domain.ConflictError{Expected: cmd.ExpectedVersion, Actual: 0}
	}
	if s.store.CaseNumberExists(strings.TrimSpace(cmd.CaseNumber)) {
		return nil, &domain.FieldError{Field: "caseNumber", Message: "编号已存在"}
	}
	now := s.now()
	c := &domain.AcceptanceCase{ID: newID("case"), CaseNumber: strings.TrimSpace(cmd.CaseNumber), SiteName: strings.TrimSpace(cmd.SiteName), ResponsibleEngineer: strings.TrimSpace(cmd.ResponsibleEngineer), Status: domain.StatusDraft, Version: 1, CreatedAt: now, UpdatedAt: now, RuleVersion: "VC-ACCEPT-2026.1", Zones: []domain.BroadcastZone{}, Points: []domain.MeasurementPoint{}, Rounds: []domain.MeasurementRound{}, Deviations: []domain.Deviation{}, Reviews: []domain.ReviewRecord{}}
	for _, in := range cmd.Zones {
		id := in.ID
		if id == "" {
			id = newID("zone")
		}
		c.Zones = append(c.Zones, domain.BroadcastZone{ID: id, CaseID: c.ID, Name: strings.TrimSpace(in.Name), UsageClass: strings.TrimSpace(in.UsageClass), AreaSquareMeters: in.AreaSquareMeters, MinimumPointCount: in.MinimumPointCount, IntelligibilityThreshold: in.IntelligibilityThreshold, Notes: strings.TrimSpace(in.Notes)})
	}
	if err := domain.ValidateNewCase(c); err != nil {
		return nil, err
	}
	if err := commit(s, c, "create_case", cmd.IdempotencyKey, cmd.Actor, "case.created", map[string]any{"caseNumber": c.CaseNumber}); err != nil {
		return nil, err
	}
	return c, nil
}

type UpdateScopeCommand struct {
	CaseID                                               string
	ExpectedVersion                                      int64
	IdempotencyKey, Actor, SiteName, ResponsibleEngineer string
	Zones                                                []ZoneInput
}

func (s *Service) UpdateScope(cmd UpdateScopeCommand) (*domain.AcceptanceCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok, err := s.existing(cmd.IdempotencyKey, "update_scope"); ok || err != nil {
		return c, err
	}
	c, err := s.loadForWrite(cmd.CaseID, cmd.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	if c.Status != domain.StatusDraft {
		return nil, domain.ErrStateConflict
	}
	oldZones := map[string]bool{}
	oldByID := map[string]domain.BroadcastZone{}
	for _, z := range c.Zones {
		oldZones[z.ID] = true
		oldByID[z.ID] = z
	}
	c.SiteName = strings.TrimSpace(cmd.SiteName)
	c.ResponsibleEngineer = strings.TrimSpace(cmd.ResponsibleEngineer)
	zones := []domain.BroadcastZone{}
	for _, in := range cmd.Zones {
		id := in.ID
		if id == "" {
			id = newID("zone")
		}
		zones = append(zones, domain.BroadcastZone{ID: id, CaseID: c.ID, Name: strings.TrimSpace(in.Name), UsageClass: strings.TrimSpace(in.UsageClass), AreaSquareMeters: in.AreaSquareMeters, MinimumPointCount: in.MinimumPointCount, IntelligibilityThreshold: in.IntelligibilityThreshold, Notes: strings.TrimSpace(in.Notes)})
	}
	c.Zones = zones
	if err = domain.ValidateNewCase(c); err != nil {
		return nil, err
	}
	for _, point := range c.Points {
		if _, ok := c.FindZone(point.ZoneID); !ok {
			return nil, &domain.FieldError{Field: "zones", Message: "不能删除已有测点所属的区域"}
		}
	}
	c.Touch(s.now())
	added, removed := 0, 0
	for _, z := range c.Zones {
		if !oldZones[z.ID] {
			added++
		}
	}
	for id := range oldZones {
		found := false
		for _, z := range c.Zones {
			if z.ID == id {
				found = true
				break
			}
		}
		if !found {
			removed++
		}
	}
	modified := 0
	for _, z := range c.Zones {
		if old, ok := oldByID[z.ID]; ok && (old.Name != z.Name || old.UsageClass != z.UsageClass || old.Notes != z.Notes || old.AreaSquareMeters != z.AreaSquareMeters || old.MinimumPointCount != z.MinimumPointCount || old.IntelligibilityThreshold != z.IntelligibilityThreshold) {
			modified++
		}
	}
	if err = commit(s, c, "update_scope", cmd.IdempotencyKey, cmd.Actor, "case.scope_updated", map[string]any{"zoneCount": len(c.Zones), "added": added, "removed": removed, "modified": modified, "order": c.Zones}); err != nil {
		return nil, err
	}
	return c, nil
}
