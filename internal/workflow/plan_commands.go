package workflow

import (
	"sort"
	"strings"
	"voice-clarity-acceptance/internal/domain"
	"voice-clarity-acceptance/internal/measurement"
)

func clonePlanRevisions(in []domain.PlanRevisionSnapshot) []domain.PlanRevisionSnapshot {
	if in == nil {
		return nil
	}
	out := make([]domain.PlanRevisionSnapshot, len(in))
	for i := range in {
		out[i] = in[i]
		if len(in[i].Points) > 0 {
			out[i].Points = make([]domain.MeasurementPoint, len(in[i].Points))
			copy(out[i].Points, in[i].Points)
		}
	}
	return out
}

type PointInput struct{ ID, ZoneID, PointCode, LocationDescription, CoverageTag string }
type SavePlanCommand struct {
	CaseID                string
	ExpectedVersion       int64
	IdempotencyKey, Actor string
	Points                []PointInput
}

func (s *Service) SavePlan(cmd SavePlanCommand) (*domain.AcceptanceCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok, err := s.existing(cmd.IdempotencyKey, "save_plan"); ok || err != nil {
		return c, err
	}
	c, err := s.loadForWrite(cmd.CaseID, cmd.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	points := make([]domain.MeasurementPoint, 0, len(cmd.Points))
	for _, in := range cmd.Points {
		id := in.ID
		if id == "" {
			id = newID("point")
		}
		points = append(points, domain.MeasurementPoint{ID: id, ZoneID: in.ZoneID, PointCode: strings.TrimSpace(in.PointCode), LocationDescription: strings.TrimSpace(in.LocationDescription), CoverageTag: strings.TrimSpace(in.CoverageTag)})
	}
	if err = c.ReplacePlan(points, s.now()); err != nil {
		return nil, err
	}
	digest, err := measurement.PlanDigest(c)
	if err != nil {
		return nil, err
	}
	c.PlanRevisions = append(c.PlanRevisions, domain.PlanRevisionSnapshot{Revision: c.PlanRevision, Points: append([]domain.MeasurementPoint(nil), c.Points...), SubmittedBy: cmd.Actor, SubmittedAt: s.now(), Digest: digest})
	if err = commit(s, c, "save_plan", cmd.IdempotencyKey, cmd.Actor, "plan.saved", map[string]any{"pointCount": len(points), "revision": c.PlanRevision}); err != nil {
		return nil, err
	}
	delete(s.planRevisionCache, c.ID)
	return c, nil
}

type PlanRevisionDiff struct {
	Added     []domain.MeasurementPoint `json:"added"`
	Removed   []domain.MeasurementPoint `json:"removed"`
	Moved     []domain.MeasurementPoint `json:"moved"`
	Changed   []domain.MeasurementPoint `json:"changed"`
	ZoneDelta map[string]int            `json:"zoneDelta"`
}

func (s *Service) PlanRevisions(caseID string) ([]domain.PlanRevisionSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if revisions, ok := s.planRevisionCache[caseID]; ok {
		return clonePlanRevisions(revisions), nil
	}
	c, e := s.store.GetCase(caseID)
	if e != nil {
		return nil, e
	}
	cached := clonePlanRevisions(c.PlanRevisions)
	s.planRevisionCache[caseID] = cached
	return clonePlanRevisions(cached), nil
}
func (s *Service) ComparePlanRevisions(caseID string, a, b int) (PlanRevisionDiff, error) {
	c, e := s.store.GetCase(caseID)
	if e != nil {
		return PlanRevisionDiff{}, e
	}
	var x, y *domain.PlanRevisionSnapshot
	for i := range c.PlanRevisions {
		if c.PlanRevisions[i].Revision == a {
			x = &c.PlanRevisions[i]
		}
		if c.PlanRevisions[i].Revision == b {
			y = &c.PlanRevisions[i]
		}
	}
	if x == nil || y == nil {
		return PlanRevisionDiff{}, &domain.FieldError{Field: "revision", Message: "修订不存在"}
	}
	old, newm := map[string]domain.MeasurementPoint{}, map[string]domain.MeasurementPoint{}
	for _, p := range x.Points {
		old[p.ID] = p
	}
	for _, p := range y.Points {
		newm[p.ID] = p
	}
	d := PlanRevisionDiff{ZoneDelta: map[string]int{}}
	for id, p := range newm {
		if q, ok := old[id]; !ok {
			d.Added = append(d.Added, p)
		} else {
			if q.ZoneID != p.ZoneID {
				d.Moved = append(d.Moved, p)
			} else if q.LocationDescription != p.LocationDescription || q.CoverageTag != p.CoverageTag || q.PointCode != p.PointCode {
				d.Changed = append(d.Changed, p)
			}
			d.ZoneDelta[q.ZoneID]--
			d.ZoneDelta[p.ZoneID]++
		}
	}
	for id, p := range old {
		if _, ok := newm[id]; !ok {
			d.Removed = append(d.Removed, p)
			d.ZoneDelta[p.ZoneID]--
		}
	}
	sort.Slice(d.Added, func(i, j int) bool { return d.Added[i].ID < d.Added[j].ID })
	sort.Slice(d.Removed, func(i, j int) bool { return d.Removed[i].ID < d.Removed[j].ID })
	sort.Slice(d.Moved, func(i, j int) bool { return d.Moved[i].ID < d.Moved[j].ID })
	sort.Slice(d.Changed, func(i, j int) bool { return d.Changed[i].ID < d.Changed[j].ID })
	return d, nil
}

type RestorePlanCommand struct {
	CaseID                string
	ExpectedVersion       int64
	IdempotencyKey, Actor string
	Revision              int
}

func (s *Service) RestorePlan(cmd RestorePlanCommand) (*domain.AcceptanceCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok, e := s.existing(cmd.IdempotencyKey, "restore_plan"); ok || e != nil {
		return c, e
	}
	c, e := s.loadForWrite(cmd.CaseID, cmd.ExpectedVersion)
	if e != nil {
		return nil, e
	}
	var snap *domain.PlanRevisionSnapshot
	for i := range c.PlanRevisions {
		if c.PlanRevisions[i].Revision == cmd.Revision {
			snap = &c.PlanRevisions[i]
			break
		}
	}
	if snap == nil {
		return nil, &domain.FieldError{Field: "revision", Message: "修订不存在"}
	}
	if e = domain.ValidatePoints(c, snap.Points); e != nil {
		return nil, e
	}
	if e = c.ReplacePlan(append([]domain.MeasurementPoint(nil), snap.Points...), s.now()); e != nil {
		return nil, e
	}
	digest, _ := measurement.PlanDigest(c)
	c.PlanRevisions = append(c.PlanRevisions, domain.PlanRevisionSnapshot{Revision: c.PlanRevision, Points: append([]domain.MeasurementPoint(nil), c.Points...), SubmittedBy: cmd.Actor, SubmittedAt: s.now(), Digest: digest})
	if e = commit(s, c, "restore_plan", cmd.IdempotencyKey, cmd.Actor, "plan.restored", map[string]any{"sourceRevision": cmd.Revision}); e != nil {
		return nil, e
	}
	delete(s.planRevisionCache, c.ID)
	return c, nil
}

type FreezePlanCommand struct {
	CaseID                string
	ExpectedVersion       int64
	IdempotencyKey, Actor string
	PlanRevision          int
	CandidateSummary      string
}

func (s *Service) PrecheckPlan(caseID string) (measurement.PlanPrecheck, error) {
	c, err := s.store.GetCase(caseID)
	if err != nil {
		return measurement.PlanPrecheck{}, err
	}
	if c.Status != domain.StatusDraft {
		return measurement.PlanPrecheck{}, domain.ErrStateConflict
	}
	return measurement.Precheck(c)
}

func (s *Service) FreezePlan(cmd FreezePlanCommand) (*domain.AcceptanceCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok, err := s.existing(cmd.IdempotencyKey, "freeze_plan"); ok || err != nil {
		return c, err
	}
	c, err := s.loadForWrite(cmd.CaseID, cmd.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	issues := measurement.ValidatePlan(c)
	if len(issues) > 0 {
		return nil, &domain.FieldError{Field: "plan", Message: issues[0].Message}
	}
	precheck, _ := measurement.Precheck(c)
	if cmd.PlanRevision != 0 && cmd.PlanRevision != precheck.Revision {
		return nil, &domain.FieldError{Field: "planRevision", Message: "计划已变化，请重新预检"}
	}
	if cmd.CandidateSummary != "" && cmd.CandidateSummary != precheck.CandidateSummary {
		return nil, &domain.FieldError{Field: "candidateSummary", Message: "计划摘要已变化，请重新预检"}
	}
	digest, err := measurement.PlanDigest(c)
	if err != nil {
		return nil, err
	}
	targets := make([]string, 0, len(c.Points))
	for _, p := range c.Points {
		targets = append(targets, p.ID)
	}
	round := domain.MeasurementRound{ID: newID("round"), CaseID: c.ID, Number: 1, Kind: "initial", Status: domain.RoundOpen, TargetPointIDs: targets, Readings: []domain.MeasurementReading{}, CreatedAt: s.now()}
	if err = c.FreezePlan(digest, round, s.now()); err != nil {
		return nil, err
	}
	c.PlanCandidateSummary = precheck.CandidateSummary
	c.PlanPrecheckRevision = precheck.Revision
	if err = commit(s, c, "freeze_plan", cmd.IdempotencyKey, cmd.Actor, "plan.frozen", map[string]any{"planDigest": digest, "roundID": round.ID, "planRevision": precheck.Revision, "candidateSummary": precheck.CandidateSummary, "issueCount": 0}); err != nil {
		return nil, err
	}
	return c, nil
}
