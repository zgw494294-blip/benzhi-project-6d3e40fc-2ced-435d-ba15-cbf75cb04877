package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"
	"voice-clarity-acceptance/internal/domain"
	"voice-clarity-acceptance/internal/measurement"
)

type AddReadingCommand struct {
	CaseID, RoundID, PointID                                    string
	ExpectedVersion                                             int64
	IdempotencyKey, Actor                                       string
	BackgroundNoiseDBA, BroadcastLevelDBA, IntelligibilityValue float64
	InstrumentID                                                string
	MeasuredAt                                                  time.Time
	Context                                                     context.Context
}

type ReadingEntry struct {
	PointID                                                     string
	BackgroundNoiseDBA, BroadcastLevelDBA, IntelligibilityValue float64
	InstrumentID                                                string
	MeasuredAt                                                  time.Time
}

type AddReadingsCommand struct {
	CaseID, RoundID, IdempotencyKey, Actor string
	ExpectedVersion                        int64
	Entries                                []ReadingEntry
}

type BatchReadingResult struct {
	Case      *domain.AcceptanceCase `json:"case"`
	Submitted int                    `json:"submitted"`
	Invalid   int                    `json:"invalid"`
	Remaining int                    `json:"remaining"`
}

func (s *Service) RetestCandidates(caseID, zoneID string) ([]measurement.RetestCandidate, error) {
	c, err := s.store.GetCase(caseID)
	if err != nil {
		return nil, err
	}
	return measurement.Candidates(c, zoneID)
}

func (s *Service) AddReading(cmd AddReadingCommand) (*domain.AcceptanceCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok, err := s.existing(cmd.IdempotencyKey, "add_reading"); ok || err != nil {
		return c, err
	}
	c, err := s.loadForWrite(cmd.CaseID, cmd.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	if _, ok := c.FindPoint(cmd.PointID); !ok {
		return nil, &domain.FieldError{Field: "pointID", Message: "未知测点"}
	}
	input := measurement.ReadingInput{BackgroundNoiseDBA: cmd.BackgroundNoiseDBA, BroadcastLevelDBA: cmd.BroadcastLevelDBA, IntelligibilityValue: cmd.IntelligibilityValue, InstrumentID: cmd.InstrumentID, MeasuredAt: cmd.MeasuredAt}
	if err = measurement.ValidateReadingFields(input, s.now()); err != nil {
		return nil, err
	}
	r := measurement.BuildReading(newID("reading"), cmd.RoundID, cmd.PointID, input, s.now())
	r.Revision = 1
	r.Operator = cmd.Actor
	if err = c.AddReading(cmd.RoundID, r, s.now()); err != nil {
		return nil, err
	}
	if err = commit(s, c, "add_reading", cmd.IdempotencyKey, cmd.Actor, "reading.recorded", map[string]any{"roundID": cmd.RoundID, "pointID": cmd.PointID, "validity": r.ValidityStatus}); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) AddReadings(cmd AddReadingsCommand) (*BatchReadingResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if prior, ok, err := s.existing(cmd.IdempotencyKey, "add_readings"); ok || err != nil {
		if err != nil {
			return nil, err
		}
		return &BatchReadingResult{Case: prior}, nil
	}
	c, err := s.loadForWrite(cmd.CaseID, cmd.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	if len(cmd.Entries) == 0 {
		return nil, &domain.FieldError{Field: "readings", Message: "至少提交一条读数"}
	}
	round, ok := c.FindRound(cmd.RoundID)
	if !ok {
		return nil, domain.ErrNotFound
	}
	if round.Status != domain.RoundOpen {
		return nil, domain.ErrStateConflict
	}
	seen := map[string]bool{}
	for _, old := range round.Readings {
		seen[old.PointID] = true
	}
	newReadings := make([]domain.MeasurementReading, 0, len(cmd.Entries))
	invalid := 0
	for i, e := range cmd.Entries {
		if seen[e.PointID] {
			return nil, &domain.FieldError{Field: fmt.Sprintf("readings[%d].pointID", i), Message: "测点重复或已有读数"}
		}
		p, exists := c.FindPoint(e.PointID)
		if !exists {
			return nil, &domain.FieldError{Field: fmt.Sprintf("readings[%d].pointID", i), Message: "未知测点"}
		}
		target := false
		for _, id := range round.TargetPointIDs {
			if id == p.ID {
				target = true
				break
			}
		}
		if !target {
			return nil, &domain.FieldError{Field: fmt.Sprintf("readings[%d].pointID", i), Message: "测点不在本轮范围"}
		}
		input := measurement.ReadingInput{BackgroundNoiseDBA: e.BackgroundNoiseDBA, BroadcastLevelDBA: e.BroadcastLevelDBA, IntelligibilityValue: e.IntelligibilityValue, InstrumentID: e.InstrumentID, MeasuredAt: e.MeasuredAt}
		if err := measurement.ValidateReadingFields(input, s.now()); err != nil {
			return nil, &domain.FieldError{Field: fmt.Sprintf("readings[%d]", i), Message: err.Error()}
		}
		r := measurement.BuildReading(newID("reading"), cmd.RoundID, e.PointID, input, s.now())
		r.Revision = 1
		r.Operator = cmd.Actor
		if r.ValidityStatus == domain.ReadingInvalid {
			invalid++
		}
		newReadings = append(newReadings, r)
		seen[e.PointID] = true
	}
	round.Readings = append(round.Readings, newReadings...)
	c.Status = domain.StatusMeasuring
	c.Touch(s.now())
	if err := commit(s, c, "add_readings", cmd.IdempotencyKey, cmd.Actor, "readings.batch_recorded", map[string]any{"roundID": cmd.RoundID, "submitted": len(newReadings), "invalid": invalid}); err != nil {
		return nil, err
	}
	remaining := 0
	for _, id := range round.TargetPointIDs {
		found := false
		for _, r := range round.Readings {
			if r.PointID == id {
				found = true
				break
			}
		}
		if !found {
			remaining++
		}
	}
	return &BatchReadingResult{Case: c, Submitted: len(newReadings), Invalid: invalid, Remaining: remaining}, nil
}

type CorrectReadingCommand struct {
	CaseID, RoundID, ReadingID, Reason, Actor, IdempotencyKey string
	ExpectedVersion                                           int64
	ReadingEntry
}

func (s *Service) CorrectReading(cmd CorrectReadingCommand) (*domain.AcceptanceCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok, err := s.existing(cmd.IdempotencyKey, "correct_reading"); ok || err != nil {
		return c, err
	}
	c, err := s.loadForWrite(cmd.CaseID, cmd.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cmd.Reason) == "" {
		return nil, &domain.FieldError{Field: "reason", Message: "纠错原因不能为空"}
	}
	round, ok := c.FindRound(cmd.RoundID)
	if !ok {
		return nil, domain.ErrNotFound
	}
	var pointID string
	for _, r := range round.Readings {
		if r.ID == cmd.ReadingID {
			pointID = r.PointID
			break
		}
	}
	if pointID == "" {
		return nil, domain.ErrNotFound
	}
	if cmd.PointID != "" && cmd.PointID != pointID {
		return nil, &domain.FieldError{Field: "pointID", Message: "替代读数必须属于原测点"}
	}
	if _, ok := c.FindPoint(pointID); !ok {
		return nil, domain.ErrNotFound
	}
	input := measurement.ReadingInput{BackgroundNoiseDBA: cmd.BackgroundNoiseDBA, BroadcastLevelDBA: cmd.BroadcastLevelDBA, IntelligibilityValue: cmd.IntelligibilityValue, InstrumentID: cmd.InstrumentID, MeasuredAt: cmd.MeasuredAt}
	if err = measurement.ValidateReadingFields(input, s.now()); err != nil {
		return nil, err
	}
	replacement := measurement.BuildReading(newID("reading"), cmd.RoundID, pointID, input, s.now())
	if err = c.CorrectReading(cmd.RoundID, cmd.ReadingID, replacement, strings.TrimSpace(cmd.Reason), cmd.Actor, s.now()); err != nil {
		return nil, err
	}
	if err = commit(s, c, "correct_reading", cmd.IdempotencyKey, cmd.Actor, "reading.corrected", map[string]any{"roundID": cmd.RoundID, "oldReadingID": cmd.ReadingID, "newReadingID": replacement.ID, "reason": cmd.Reason}); err != nil {
		return nil, err
	}
	return c, nil
}

type CloseRoundCommand struct {
	CaseID, RoundID       string
	ExpectedVersion       int64
	IdempotencyKey, Actor string
}

func (s *Service) CloseRound(cmd CloseRoundCommand) (*domain.AcceptanceCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok, err := s.existing(cmd.IdempotencyKey, "close_round"); ok || err != nil {
		return c, err
	}
	c, err := s.loadForWrite(cmd.CaseID, cmd.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	_, ok := c.FindRound(cmd.RoundID)
	if !ok {
		return nil, domain.ErrNotFound
	}
	gate, ge := measurement.Gate(c, cmd.RoundID, s.now())
	if ge != nil {
		return nil, ge
	}
	if !gate.CanClose {
		missing := []string{}
		for _, i := range gate.Items {
			if i.Status == "missing" {
				missing = append(missing, i.PointID)
			}
		}
		if len(missing) > 0 {
			return nil, &domain.FieldError{Field: "readings", Message: "本轮仍有缺失测点: " + strings.Join(missing, ",")}
		}
		return nil, &domain.FieldError{Field: "qualityGate", Message: "存在证据冲突，不能关闭轮次"}
	}
	decision := measurement.Calculate(c, s.now())
	if err = c.CloseRound(cmd.RoundID, decision, s.now()); err != nil {
		return nil, err
	}
	if err = commit(s, c, "close_round", cmd.IdempotencyKey, cmd.Actor, "round.closed", map[string]any{"roundID": cmd.RoundID, "passed": decision.Passed, "failedZoneIDs": decision.FailedZoneIDs}); err != nil {
		return nil, err
	}
	return c, nil
}

type CreateDeviationCommand struct {
	CaseID, ZoneID, Reason, CorrectiveAction string
	TargetPointIDs                           []string
	ExpectedVersion                          int64
	IdempotencyKey, Actor                    string
}

func (s *Service) CreateDeviation(cmd CreateDeviationCommand) (*domain.AcceptanceCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok, err := s.existing(cmd.IdempotencyKey, "create_deviation"); ok || err != nil {
		return c, err
	}
	c, err := s.loadForWrite(cmd.CaseID, cmd.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cmd.Reason) == "" || strings.TrimSpace(cmd.CorrectiveAction) == "" {
		return nil, &domain.FieldError{Field: "deviation", Message: "原因和整改措施不能为空"}
	}
	if _, ok := c.FindZone(cmd.ZoneID); !ok {
		return nil, domain.ErrNotFound
	}
	targets, err := measurement.RetestTargets(c, cmd.ZoneID, cmd.TargetPointIDs)
	if err != nil {
		return nil, err
	}
	now := s.now()
	d := domain.Deviation{ID: newID("deviation"), CaseID: c.ID, ZoneID: cmd.ZoneID, Reason: strings.TrimSpace(cmd.Reason), CorrectiveAction: strings.TrimSpace(cmd.CorrectiveAction), TargetPointIDs: targets, Status: domain.DeviationOpen, OpenedAt: now}
	if c.Decision != nil {
		d.SourceDecisionVersion = c.Version
		d.SourceRuleVersion = c.Decision.RuleVersion
	}
	round := domain.MeasurementRound{ID: newID("round"), CaseID: c.ID, Number: len(c.Rounds) + 1, Kind: "retest", Status: domain.RoundOpen, TargetPointIDs: targets, Readings: []domain.MeasurementReading{}, CreatedAt: now}
	if err = c.AddDeviation(d, round, now); err != nil {
		return nil, err
	}
	if err = commit(s, c, "create_deviation", cmd.IdempotencyKey, cmd.Actor, "deviation.opened", map[string]any{"deviationID": d.ID, "roundID": round.ID, "targets": targets, "sourceDecisionVersion": d.SourceDecisionVersion, "sourceRuleVersion": d.SourceRuleVersion}); err != nil {
		return nil, err
	}
	return c, nil
}
