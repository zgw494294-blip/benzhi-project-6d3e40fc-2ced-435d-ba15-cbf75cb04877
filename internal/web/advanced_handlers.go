package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"
	"voice-clarity-acceptance/internal/domain"
	"voice-clarity-acceptance/internal/workflow"
)

func (s *Server) PlanRevisionsHandler(w http.ResponseWriter, r *http.Request) {
	x, e := s.service.PlanRevisions(r.PathValue("caseID"))
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": x})
}
func (s *Server) ComparePlanRevisionsHandler(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	if from == "" {
		from = r.URL.Query().Get("fromRevision")
	}
	to := r.URL.Query().Get("to")
	if to == "" {
		to = r.URL.Query().Get("toRevision")
	}
	a, e1 := strconv.Atoi(from)
	b, e2 := strconv.Atoi(to)
	if e1 != nil || e2 != nil {
		writeError(w, &domain.FieldError{Field: "revision", Message: "from 和 to 必须是整数"})
		return
	}
	x, e := s.service.ComparePlanRevisions(r.PathValue("caseID"), a, b)
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, http.StatusOK, x)
}
func (s *Server) RestorePlanHandler(w http.ResponseWriter, r *http.Request) {
	var body mutationMeta
	if !decodeJSON(w, r, &body) {
		return
	}
	rev, e := strconv.Atoi(r.PathValue("revision"))
	if e != nil {
		writeError(w, &domain.FieldError{Field: "revision", Message: "必须是整数"})
		return
	}
	x, e := s.service.RestorePlan(workflow.RestorePlanCommand{CaseID: r.PathValue("caseID"), Revision: rev, ExpectedVersion: body.ExpectedVersion, IdempotencyKey: body.IdempotencyKey, Actor: actor(r)})
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, http.StatusOK, x)
}
func (s *Server) QualityGateHandler(w http.ResponseWriter, r *http.Request) {
	x, e := s.service.QualityGate(r.PathValue("caseID"), r.PathValue("roundID"))
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, http.StatusOK, x)
}
func (s *Server) CaseQualityGateHandler(w http.ResponseWriter, r *http.Request) {
	roundID := r.URL.Query().Get("roundID")
	if roundID == "" {
		writeError(w, &domain.FieldError{Field: "roundID", Message: "不能为空"})
		return
	}
	x, e := s.service.QualityGate(r.PathValue("caseID"), roundID)
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, http.StatusOK, x)
}
func (s *Server) DecisionReviewHandler(w http.ResponseWriter, r *http.Request) {
	x, e := s.service.ReviewGrades(r.PathValue("caseID"), strings.TrimSpace(r.URL.Query().Get("zoneID")), strings.TrimSpace(r.URL.Query().Get("grade")), strings.TrimSpace(r.URL.Query().Get("roundID")))
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, http.StatusOK, x)
}
func (s *Server) AddExecutionHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		mutationMeta
		Content     string    `json:"content"`
		Operator    string    `json:"operator"`
		CompletedAt time.Time `json:"completedAt"`
		PointIDs    []string  `json:"pointIDs"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Operator) == "" {
		body.Operator = actor(r)
	}
	x, e := s.service.AddExecution(workflow.ExecutionCommand{CaseID: r.PathValue("caseID"), DeviationID: r.PathValue("deviationID"), Content: body.Content, Operator: body.Operator, CompletedAt: body.CompletedAt, PointIDs: body.PointIDs, ExpectedVersion: body.ExpectedVersion, IdempotencyKey: body.IdempotencyKey})
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, http.StatusOK, x)
}
func (s *Server) ResolveIssueHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		mutationMeta
		Explanation string `json:"explanation"`
		Resolver    string `json:"resolver"`
		DeviationID string `json:"deviationID"`
		RoundID     string `json:"roundID"`
		ReadingID   string `json:"readingID"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Resolver) == "" {
		body.Resolver = actor(r)
	}
	x, e := s.service.ResolveIssue(workflow.ResolveIssueCommand{CaseID: r.PathValue("caseID"), IssueID: r.PathValue("issueID"), Explanation: body.Explanation, Resolver: body.Resolver, DeviationID: body.DeviationID, RoundID: body.RoundID, ReadingID: body.ReadingID, ExpectedVersion: body.ExpectedVersion, IdempotencyKey: body.IdempotencyKey})
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, http.StatusOK, x)
}
func (s *Server) ApprovalGateHandler(w http.ResponseWriter, r *http.Request) {
	x, e := s.service.ApprovalGate(r.PathValue("caseID"))
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, http.StatusOK, x)
}
func (s *Server) LookupCredentialHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("credentialID")
	seq := uint64(0)
	if raw := r.URL.Query().Get("sequence"); raw != "" {
		n, e := strconv.ParseUint(raw, 10, 64)
		if e != nil {
			writeError(w, &domain.FieldError{Field: "sequence", Message: "必须是正整数"})
			return
		}
		seq = n
	}
	x, e := s.service.LookupCredential(id, seq)
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, http.StatusOK, x)
}
func (s *Server) VerifyCredentialLookupHandler(w http.ResponseWriter, r *http.Request) {
	x, e := s.service.LookupCredential(r.PathValue("credentialID"), 0)
	if e != nil {
		writeError(w, e)
		return
	}
	v, e := s.service.VerifyCredential(x.CaseID)
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (s *Server) LookupCredentialPathHandler(w http.ResponseWriter, r *http.Request) {
	x, e := s.service.LookupCredential(r.PathValue("credentialID"), 0)
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, http.StatusOK, x)
}
