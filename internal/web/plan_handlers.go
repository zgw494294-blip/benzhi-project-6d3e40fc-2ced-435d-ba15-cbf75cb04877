package web

import (
	"net/http"
	"voice-clarity-acceptance/internal/workflow"
)

type pointRequest struct {
	ID                  string `json:"id"`
	ZoneID              string `json:"zoneID"`
	PointCode           string `json:"pointCode"`
	LocationDescription string `json:"locationDescription"`
	CoverageTag         string `json:"coverageTag"`
}

func (s *Server) SavePlanHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		mutationMeta
		Points []pointRequest `json:"points"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	points := make([]workflow.PointInput, 0, len(body.Points))
	for _, p := range body.Points {
		points = append(points, workflow.PointInput{ID: p.ID, ZoneID: p.ZoneID, PointCode: p.PointCode, LocationDescription: p.LocationDescription, CoverageTag: p.CoverageTag})
	}
	c, err := s.service.SavePlan(workflow.SavePlanCommand{CaseID: r.PathValue("caseID"), ExpectedVersion: body.ExpectedVersion, IdempotencyKey: body.IdempotencyKey, Actor: actor(r), Points: points})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) FreezePlanHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		mutationMeta
		PlanRevision     int    `json:"planRevision"`
		CandidateSummary string `json:"candidateSummary"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	c, err := s.service.FreezePlan(workflow.FreezePlanCommand{CaseID: r.PathValue("caseID"), ExpectedVersion: body.ExpectedVersion, IdempotencyKey: body.IdempotencyKey, Actor: actor(r), PlanRevision: body.PlanRevision, CandidateSummary: body.CandidateSummary})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) PrecheckPlanHandler(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.PrecheckPlan(r.PathValue("caseID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
