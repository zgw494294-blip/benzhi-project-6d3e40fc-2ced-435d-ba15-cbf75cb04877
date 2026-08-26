package web

import (
	"net/http"
	"strconv"
	"strings"
	"voice-clarity-acceptance/internal/domain"
	"voice-clarity-acceptance/internal/workflow"
)

type zoneRequest struct {
	ID                       string  `json:"id"`
	Name                     string  `json:"name"`
	UsageClass               string  `json:"usageClass"`
	Notes                    string  `json:"notes"`
	AreaSquareMeters         float64 `json:"areaSquareMeters"`
	MinimumPointCount        int     `json:"minimumPointCount"`
	IntelligibilityThreshold float64 `json:"intelligibilityThreshold"`
}

func zonesFromRequest(in []zoneRequest) []workflow.ZoneInput {
	out := make([]workflow.ZoneInput, 0, len(in))
	for _, z := range in {
		out = append(out, workflow.ZoneInput{ID: z.ID, Name: z.Name, UsageClass: z.UsageClass, Notes: z.Notes, AreaSquareMeters: z.AreaSquareMeters, MinimumPointCount: z.MinimumPointCount, IntelligibilityThreshold: z.IntelligibilityThreshold})
	}
	return out
}

func (s *Server) ListCasesHandler(w http.ResponseWriter, r *http.Request) {
	page, pageSize := 1, 20
	var err error
	if raw := r.URL.Query().Get("page"); raw != "" {
		page, err = strconv.Atoi(raw)
		if err != nil {
			writeError(w, &domain.FieldError{Field: "page", Message: "必须是正整数"})
			return
		}
	}
	if raw := r.URL.Query().Get("pageSize"); raw != "" {
		pageSize, err = strconv.Atoi(raw)
		if err != nil {
			writeError(w, &domain.FieldError{Field: "pageSize", Message: "必须是整数"})
			return
		}
	}
	cases, err := s.service.QueryCases(workflow.CaseQuery{Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")), Status: strings.TrimSpace(r.URL.Query().Get("status")), Sort: strings.TrimSpace(r.URL.Query().Get("sort")), Page: page, PageSize: pageSize})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cases)
}
func (s *Server) GetCaseHandler(w http.ResponseWriter, r *http.Request) {
	c, err := s.service.GetCase(r.PathValue("caseID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) CreateCaseHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CaseNumber          string        `json:"caseNumber"`
		SiteName            string        `json:"siteName"`
		ResponsibleEngineer string        `json:"responsibleEngineer"`
		ExpectedVersion     int64         `json:"expectedVersion"`
		IdempotencyKey      string        `json:"idempotencyKey"`
		Zones               []zoneRequest `json:"zones"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	c, err := s.service.CreateCase(workflow.CreateCaseCommand{CaseNumber: body.CaseNumber, SiteName: body.SiteName, ResponsibleEngineer: body.ResponsibleEngineer, ExpectedVersion: body.ExpectedVersion, IdempotencyKey: body.IdempotencyKey, Actor: actor(r), Zones: zonesFromRequest(body.Zones)})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) UpdateScopeHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		mutationMeta
		SiteName            string        `json:"siteName"`
		ResponsibleEngineer string        `json:"responsibleEngineer"`
		Zones               []zoneRequest `json:"zones"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	c, err := s.service.UpdateScope(workflow.UpdateScopeCommand{CaseID: r.PathValue("caseID"), ExpectedVersion: body.ExpectedVersion, IdempotencyKey: body.IdempotencyKey, Actor: actor(r), SiteName: body.SiteName, ResponsibleEngineer: body.ResponsibleEngineer, Zones: zonesFromRequest(body.Zones)})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}
