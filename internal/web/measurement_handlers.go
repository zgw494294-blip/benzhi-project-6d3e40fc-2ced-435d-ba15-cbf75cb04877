package web

import (
	"net/http"
	"strings"
	"time"
	"voice-clarity-acceptance/internal/domain"
	"voice-clarity-acceptance/internal/workflow"
)

func (s *Server) RetestCandidatesHandler(w http.ResponseWriter, r *http.Request) {
	items, err := s.service.RetestCandidates(r.PathValue("caseID"), strings.TrimSpace(r.URL.Query().Get("zoneID")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) AddReadingHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		mutationMeta
		Readings []struct {
			PointID              string    `json:"pointID"`
			BackgroundNoiseDBA   float64   `json:"backgroundNoiseDBA"`
			BroadcastLevelDBA    float64   `json:"broadcastLevelDBA"`
			IntelligibilityValue float64   `json:"intelligibilityValue"`
			InstrumentID         string    `json:"instrumentID"`
			MeasuredAt           time.Time `json:"measuredAt"`
		} `json:"readings"`
		PointID              string    `json:"pointID"`
		BackgroundNoiseDBA   float64   `json:"backgroundNoiseDBA"`
		BroadcastLevelDBA    float64   `json:"broadcastLevelDBA"`
		IntelligibilityValue float64   `json:"intelligibilityValue"`
		InstrumentID         string    `json:"instrumentID"`
		MeasuredAt           time.Time `json:"measuredAt"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if len(body.Readings) > 0 {
		entries := make([]workflow.ReadingEntry, 0, len(body.Readings))
		for _, v := range body.Readings {
			entries = append(entries, workflow.ReadingEntry{PointID: v.PointID, BackgroundNoiseDBA: v.BackgroundNoiseDBA, BroadcastLevelDBA: v.BroadcastLevelDBA, IntelligibilityValue: v.IntelligibilityValue, InstrumentID: v.InstrumentID, MeasuredAt: v.MeasuredAt})
		}
		result, err := s.service.AddReadings(workflow.AddReadingsCommand{CaseID: r.PathValue("caseID"), RoundID: r.PathValue("roundID"), ExpectedVersion: body.ExpectedVersion, IdempotencyKey: body.IdempotencyKey, Actor: actor(r), Entries: entries})
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, struct {
			*domain.AcceptanceCase
			Submitted int `json:"submitted"`
			Invalid   int `json:"invalid"`
			Remaining int `json:"remaining"`
		}{result.Case, result.Submitted, result.Invalid, result.Remaining})
		return
	}
	c, err := s.service.AddReading(workflow.AddReadingCommand{CaseID: r.PathValue("caseID"), RoundID: r.PathValue("roundID"), PointID: body.PointID, ExpectedVersion: body.ExpectedVersion, IdempotencyKey: body.IdempotencyKey, Actor: actor(r), BackgroundNoiseDBA: body.BackgroundNoiseDBA, BroadcastLevelDBA: body.BroadcastLevelDBA, IntelligibilityValue: body.IntelligibilityValue, InstrumentID: body.InstrumentID, MeasuredAt: body.MeasuredAt})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) CorrectReadingHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		mutationMeta
		ReadingID            string    `json:"readingID"`
		Reason               string    `json:"reason"`
		PointID              string    `json:"pointID"`
		BackgroundNoiseDBA   float64   `json:"backgroundNoiseDBA"`
		BroadcastLevelDBA    float64   `json:"broadcastLevelDBA"`
		IntelligibilityValue float64   `json:"intelligibilityValue"`
		InstrumentID         string    `json:"instrumentID"`
		MeasuredAt           time.Time `json:"measuredAt"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	readingID := r.PathValue("readingID")
	if readingID == "" {
		readingID = body.ReadingID
	}
	c, err := s.service.CorrectReading(workflow.CorrectReadingCommand{CaseID: r.PathValue("caseID"), RoundID: r.PathValue("roundID"), ReadingID: readingID, ExpectedVersion: body.ExpectedVersion, IdempotencyKey: body.IdempotencyKey, Actor: actor(r), Reason: body.Reason, ReadingEntry: workflow.ReadingEntry{PointID: body.PointID, BackgroundNoiseDBA: body.BackgroundNoiseDBA, BroadcastLevelDBA: body.BroadcastLevelDBA, IntelligibilityValue: body.IntelligibilityValue, InstrumentID: body.InstrumentID, MeasuredAt: body.MeasuredAt}})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) CloseRoundHandler(w http.ResponseWriter, r *http.Request) {
	var body mutationMeta
	if !decodeJSON(w, r, &body) {
		return
	}
	c, err := s.service.CloseRound(workflow.CloseRoundCommand{CaseID: r.PathValue("caseID"), RoundID: r.PathValue("roundID"), ExpectedVersion: body.ExpectedVersion, IdempotencyKey: body.IdempotencyKey, Actor: actor(r)})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) CreateDeviationHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		mutationMeta
		ZoneID           string   `json:"zoneID"`
		Reason           string   `json:"reason"`
		CorrectiveAction string   `json:"correctiveAction"`
		TargetPointIDs   []string `json:"targetPointIDs"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	c, err := s.service.CreateDeviation(workflow.CreateDeviationCommand{CaseID: r.PathValue("caseID"), ZoneID: body.ZoneID, Reason: body.Reason, CorrectiveAction: body.CorrectiveAction, TargetPointIDs: body.TargetPointIDs, ExpectedVersion: body.ExpectedVersion, IdempotencyKey: body.IdempotencyKey, Actor: actor(r)})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}
