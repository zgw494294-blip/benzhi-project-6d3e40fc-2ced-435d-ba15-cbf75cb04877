package workflow

import (
	"strings"
	"time"
	"voice-clarity-acceptance/internal/domain"
	"voice-clarity-acceptance/internal/measurement"
)

func (s *Service) QualityGate(caseID, roundID string) (measurement.QualityGate, error) {
	c, e := s.store.GetCase(caseID)
	if e != nil {
		return measurement.QualityGate{}, e
	}
	return measurement.Gate(c, roundID, s.now())
}
func (s *Service) ReviewGrades(caseID, zoneID, grade, roundID string) (measurement.ReviewGrades, error) {
	c, e := s.store.GetCase(caseID)
	if e != nil {
		return measurement.ReviewGrades{}, e
	}
	return measurement.Grade(c, zoneID, grade, roundID)
}

type ExecutionCommand struct {
	CaseID, DeviationID, Content, Operator string
	CompletedAt                            time.Time
	PointIDs                               []string
	ExpectedVersion                        int64
	IdempotencyKey                         string
}

func (s *Service) AddExecution(cmd ExecutionCommand) (*domain.AcceptanceCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok, e := s.existing(cmd.IdempotencyKey, "add_execution"); ok || e != nil {
		return c, e
	}
	c, e := s.loadForWrite(cmd.CaseID, cmd.ExpectedVersion)
	if e != nil {
		return nil, e
	}
	var d *domain.Deviation
	for i := range c.Deviations {
		if c.Deviations[i].ID == cmd.DeviationID {
			d = &c.Deviations[i]
			break
		}
	}
	if d == nil {
		return nil, domain.ErrNotFound
	}
	if d.Status != domain.DeviationOpen {
		return nil, domain.ErrStateConflict
	}
	if strings.TrimSpace(cmd.Content) == "" {
		return nil, &domain.FieldError{Field: "content", Message: "执行内容不能为空"}
	}
	if len(cmd.PointIDs) == 0 {
		return nil, &domain.FieldError{Field: "pointIDs", Message: "至少关联一个受影响测点"}
	}
	if cmd.CompletedAt.IsZero() || cmd.CompletedAt.After(s.now()) {
		return nil, &domain.FieldError{Field: "completedAt", Message: "完成时间不能晚于当前时间"}
	}
	roundOK := false
	for _, r := range c.Rounds {
		if r.Kind == "retest" && r.Status == domain.RoundOpen && sameTargets(r.TargetPointIDs, d.TargetPointIDs) {
			roundOK = true
			break
		}
	}
	if !roundOK {
		return nil, &domain.FieldError{Field: "pointIDs", Message: "偏差尚无对应的开放定向复测范围"}
	}
	seen := map[string]bool{}
	for _, id := range cmd.PointIDs {
		if seen[id] {
			return nil, &domain.FieldError{Field: "pointIDs", Message: "目标测点重复"}
		}
		seen[id] = true
		p, ok := c.FindPoint(id)
		if !ok || p.ZoneID != d.ZoneID {
			return nil, &domain.FieldError{Field: "pointIDs", Message: "目标测点不属于偏差区域"}
		}
		found := false
		for _, target := range d.TargetPointIDs {
			if target == id {
				found = true
			}
		}
		if !found {
			return nil, &domain.FieldError{Field: "pointIDs", Message: "目标测点不在定向复测范围"}
		}
	}
	d.ExecutionRecords = append(d.ExecutionRecords, domain.ExecutionRecord{ID: newID("execution"), DeviationID: d.ID, Content: strings.TrimSpace(cmd.Content), Operator: strings.TrimSpace(cmd.Operator), CompletedAt: cmd.CompletedAt, PointIDs: append([]string(nil), cmd.PointIDs...)})
	c.Touch(s.now())
	if e = commit(s, c, "add_execution", cmd.IdempotencyKey, cmd.Operator, "deviation.execution_added", map[string]any{"deviationID": d.ID, "pointIDs": cmd.PointIDs}); e != nil {
		return nil, e
	}
	return c, nil
}
func sameTargets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	am := map[string]bool{}
	for _, x := range a {
		am[x] = true
	}
	for _, x := range b {
		if !am[x] {
			return false
		}
	}
	return true
}

type ResolveIssueCommand struct {
	CaseID, IssueID, Explanation, Resolver, DeviationID, RoundID, ReadingID string
	ExpectedVersion                                                         int64
	IdempotencyKey                                                          string
}

func (s *Service) ResolveIssue(cmd ResolveIssueCommand) (*domain.AcceptanceCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok, e := s.existing(cmd.IdempotencyKey, "resolve_review_issue"); ok || e != nil {
		return c, e
	}
	c, e := s.loadForWrite(cmd.CaseID, cmd.ExpectedVersion)
	if e != nil {
		return nil, e
	}
	if strings.TrimSpace(cmd.Explanation) == "" {
		return nil, &domain.FieldError{Field: "explanation", Message: "处理说明不能为空"}
	}
	if strings.TrimSpace(cmd.Resolver) == "" {
		return nil, &domain.FieldError{Field: "resolver", Message: "解决人不能为空"}
	}
	if cmd.DeviationID == "" && cmd.RoundID == "" && cmd.ReadingID == "" {
		return nil, &domain.FieldError{Field: "evidence", Message: "至少关联一项本案证据"}
	}
	var issue *domain.ReviewIssue
	var reviewAt time.Time
	for ri := range c.Reviews {
		for ii := range c.Reviews[ri].Issues {
			if c.Reviews[ri].Issues[ii].ID == cmd.IssueID {
				issue = &c.Reviews[ri].Issues[ii]
				reviewAt = c.Reviews[ri].ReviewedAt
			}
		}
	}
	if issue == nil {
		return nil, domain.ErrNotFound
	}
	if issue.Resolved {
		return nil, domain.ErrStateConflict
	}
	if issue.ZoneID != "" {
		if _, ok := c.FindZone(issue.ZoneID); !ok {
			return nil, &domain.FieldError{Field: "zoneID", Message: "区域不属于当前验收案"}
		}
	}
	if cmd.DeviationID != "" {
		ok := false
		for _, d := range c.Deviations {
			if d.ID == cmd.DeviationID && !d.OpenedAt.Before(reviewAt) && (issue.ZoneID == "" || d.ZoneID == issue.ZoneID) {
				if issue.PointID != "" {
					for _, pid := range d.TargetPointIDs {
						if pid == issue.PointID {
							ok = true
							break
						}
					}
				} else {
					ok = true
				}
			}
		}
		if !ok {
			return nil, &domain.FieldError{Field: "deviationID", Message: "偏差证据不属于该审核事项时间范围"}
		}
	}
	if cmd.RoundID != "" {
		ok := false
		for _, r := range c.Rounds {
			if r.ID == cmd.RoundID && !r.CreatedAt.Before(reviewAt) {
				if issue.ZoneID != "" {
					for _, pid := range r.TargetPointIDs {
						if p, exists := c.FindPoint(pid); exists && p.ZoneID == issue.ZoneID {
							ok = true
							break
						}
					}
				} else {
					ok = true
				}
			}
		}
		if !ok {
			return nil, &domain.FieldError{Field: "roundID", Message: "复测证据不属于该审核事项时间范围"}
		}
	}
	if cmd.ReadingID != "" {
		ok := false
		for _, r := range c.Rounds {
			for _, rd := range r.Readings {
				if rd.ID == cmd.ReadingID && !rd.MeasuredAt.Before(reviewAt) {
					if issue.PointID == "" || issue.PointID == rd.PointID {
						ok = true
					}
				}
			}
		}
		if !ok {
			return nil, &domain.FieldError{Field: "readingID", Message: "读数证据不属于当前事项或时间范围"}
		}
	}
	now := s.now()
	issue.Resolved = true
	issue.Resolution = &domain.IssueResolution{Resolver: cmd.Resolver, ResolvedAt: now, Explanation: strings.TrimSpace(cmd.Explanation), DeviationID: cmd.DeviationID, RoundID: cmd.RoundID, ReadingID: cmd.ReadingID}
	c.Touch(now)
	if e = commit(s, c, "resolve_review_issue", cmd.IdempotencyKey, cmd.Resolver, "review.issue_resolved", map[string]any{"issueID": cmd.IssueID}); e != nil {
		return nil, e
	}
	return c, nil
}

type ApprovalGate struct {
	Passed           bool     `json:"passed"`
	UnresolvedIssues []string `json:"unresolvedIssues"`
	OpenRounds       []string `json:"openRounds"`
	OpenDeviations   []string `json:"openDeviations"`
	Message          string   `json:"message"`
}

func (s *Service) ApprovalGate(caseID string) (ApprovalGate, error) {
	c, e := s.store.GetCase(caseID)
	if e != nil {
		return ApprovalGate{}, e
	}
	g := ApprovalGate{Passed: true}
	for _, r := range c.Reviews {
		for _, i := range r.Issues {
			if !i.Resolved {
				g.UnresolvedIssues = append(g.UnresolvedIssues, i.ID)
			}
		}
	}
	for _, r := range c.Rounds {
		if r.Status == domain.RoundOpen {
			g.OpenRounds = append(g.OpenRounds, r.ID)
		}
	}
	for _, d := range c.Deviations {
		if d.Status == domain.DeviationOpen {
			g.OpenDeviations = append(g.OpenDeviations, d.ID)
		}
	}
	g.Passed = c.Decision != nil && c.Decision.Passed && len(g.UnresolvedIssues) == 0 && len(g.OpenRounds) == 0 && len(g.OpenDeviations) == 0
	if !g.Passed {
		g.Message = "存在未解决事项、开放轮次或偏差"
	} else {
		g.Message = "批准门禁通过"
	}
	return g, nil
}

type CredentialLookup struct {
	Credential *domain.AcceptanceCredential `json:"credential"`
	CaseID     string                       `json:"caseID"`
	CaseNumber string                       `json:"caseNumber"`
	SiteName   string                       `json:"siteName"`
	Reviewer   string                       `json:"reviewer"`
	IssuedAt   time.Time                    `json:"issuedAt"`
	Found      bool                         `json:"found"`
}

func (s *Service) LookupCredential(id string, seq uint64) (CredentialLookup, error) {
	if (strings.TrimSpace(id) == "") == (seq == 0) {
		return CredentialLookup{}, &domain.FieldError{Field: "credential", Message: "必须且只能提供 credentialID 或 sequence"}
	}
	c, e := s.store.FindCredential(strings.TrimSpace(id), seq)
	if e != nil {
		return CredentialLookup{}, e
	}
	return CredentialLookup{Credential: c.Credential, CaseID: c.ID, CaseNumber: c.CaseNumber, SiteName: c.SiteName, Reviewer: c.Credential.Reviewer, IssuedAt: c.Credential.IssuedAt, Found: true}, nil
}
