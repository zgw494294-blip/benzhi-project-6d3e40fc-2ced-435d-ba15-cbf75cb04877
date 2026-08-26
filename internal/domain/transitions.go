package domain

import "time"

func (c *AcceptanceCase) Touch(now time.Time) { c.Version++; c.UpdatedAt = now }

func (c *AcceptanceCase) ReplacePlan(points []MeasurementPoint, now time.Time) error {
	if err := c.EnsureMutable(); err != nil {
		return err
	}
	if err := ValidatePoints(c, points); err != nil {
		return err
	}
	for i := range points {
		points[i].PlanRevision = c.PlanRevision + 1
		points[i].Sequence = i + 1
	}
	c.Points = points
	c.PlanRevision++
	c.Touch(now)
	return nil
}

func (c *AcceptanceCase) FreezePlan(digest string, round MeasurementRound, now time.Time) error {
	if c.Status != StatusDraft {
		return ErrStateConflict
	}
	if len(c.Points) == 0 {
		return &FieldError{"points", "测点计划为空"}
	}
	c.PlanDigest = digest
	c.Status = StatusPlanFrozen
	c.Rounds = append(c.Rounds, round)
	c.Touch(now)
	return nil
}

func (c *AcceptanceCase) AddReading(roundID string, reading MeasurementReading, now time.Time) error {
	if err := c.EnsureMutable(); err != nil {
		return err
	}
	if c.Status != StatusPlanFrozen && c.Status != StatusMeasuring && c.Status != StatusRemediation && c.Status != StatusReturned {
		return ErrStateConflict
	}
	r, ok := c.FindRound(roundID)
	if !ok {
		return ErrNotFound
	}
	if r.Status != RoundOpen {
		return ErrStateConflict
	}
	for _, existing := range r.Readings {
		if existing.PointID == reading.PointID {
			return &FieldError{"pointID", "本轮该测点已提交"}
		}
	}
	target := false
	for _, id := range r.TargetPointIDs {
		if id == reading.PointID {
			target = true
			break
		}
	}
	if !target {
		return &FieldError{"pointID", "测点不在本轮范围"}
	}
	r.Readings = append(r.Readings, reading)
	c.Status = StatusMeasuring
	c.Touch(now)
	return nil
}

func (c *AcceptanceCase) CorrectReading(roundID string, currentID string, replacement MeasurementReading, reason, operator string, now time.Time) error {
	if err := c.EnsureMutable(); err != nil {
		return err
	}
	if c.Status != StatusMeasuring && c.Status != StatusRemediation && c.Status != StatusReturned {
		return ErrStateConflict
	}
	r, ok := c.FindRound(roundID)
	if !ok {
		return ErrNotFound
	}
	if r.Status != RoundOpen {
		return ErrStateConflict
	}
	var current *MeasurementReading
	for i := range r.Readings {
		if r.Readings[i].ID == currentID {
			current = &r.Readings[i]
			break
		}
	}
	if current == nil {
		return ErrNotFound
	}
	if current.SupersededByReadingID != "" {
		return ErrEvidenceConflict
	}
	if current.PointID != replacement.PointID {
		return &FieldError{Field: "pointID", Message: "替代读数必须属于同一测点"}
	}
	replacement.Revision = current.Revision + 1
	replacement.SupersedesReadingID = current.ID
	replacement.CorrectionReason = reason
	replacement.Operator = operator
	current.SupersededByReadingID = replacement.ID
	r.Readings = append(r.Readings, replacement)
	c.Touch(now)
	return nil
}

func (c *AcceptanceCase) CloseRound(roundID string, decision CaseDecision, now time.Time) error {
	r, ok := c.FindRound(roundID)
	if !ok {
		return ErrNotFound
	}
	if r.Status != RoundOpen {
		return ErrStateConflict
	}
	r.Status, r.ClosedAt = RoundClosed, &now
	c.Decision = &decision
	if decision.Passed {
		c.Status = StatusPendingReview
	} else {
		c.Status = StatusRemediation
	}
	for i := range c.Deviations {
		if c.Deviations[i].Status == DeviationOpen && deviationSatisfied(c.Deviations[i], decision) {
			c.Deviations[i].Status, c.Deviations[i].ClosedAt = DeviationClosed, &now
		}
	}
	c.Touch(now)
	return nil
}

func deviationSatisfied(d Deviation, decision CaseDecision) bool {
	for _, target := range d.TargetPointIDs {
		found := false
		for _, p := range decision.Points {
			if p.PointID == target && p.Valid && p.Passed {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (c *AcceptanceCase) AddDeviation(d Deviation, round MeasurementRound, now time.Time) error {
	if c.Status != StatusRemediation && c.Status != StatusReturned {
		return ErrStateConflict
	}
	c.Deviations = append(c.Deviations, d)
	c.Rounds = append(c.Rounds, round)
	c.Touch(now)
	return nil
}

func (c *AcceptanceCase) Review(record ReviewRecord, credential *AcceptanceCredential, now time.Time) error {
	if c.Status != StatusPendingReview {
		return ErrStateConflict
	}
	c.Reviews = append(c.Reviews, record)
	if record.Decision == "return" {
		c.Status = StatusReturned
	} else if record.Decision == "approve" {
		if credential == nil {
			return &FieldError{"credential", "批准时必须签发凭据"}
		}
		c.Status, c.Credential = StatusApproved, credential
	} else {
		return &FieldError{"decision", "仅支持 approve 或 return"}
	}
	c.Touch(now)
	return nil
}
