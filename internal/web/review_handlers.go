package web

import "net/http"
import "voice-clarity-acceptance/internal/domain"
import "voice-clarity-acceptance/internal/workflow"

func (s *Server) ReviewHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		mutationMeta
		Decision string               `json:"decision"`
		Reviewer string               `json:"reviewer"`
		Comment  string               `json:"comment"`
		Issues   []domain.ReviewIssue `json:"issues"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	c, err := s.service.Review(workflow.ReviewCommand{CaseID: r.PathValue("caseID"), Decision: body.Decision, Reviewer: body.Reviewer, Comment: body.Comment, Issues: body.Issues, ExpectedVersion: body.ExpectedVersion, IdempotencyKey: body.IdempotencyKey})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}
func (s *Server) AuditHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := s.service.GetCase(r.PathValue("caseID")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.service.Audit(r.PathValue("caseID"))})
}
func (s *Server) CredentialHandler(w http.ResponseWriter, r *http.Request) {
	c, err := s.service.GetCase(r.PathValue("caseID"))
	if err != nil {
		writeError(w, err)
		return
	}
	if c.Credential == nil {
		writeError(w, domain.ErrCredentialMissing)
		return
	}
	writeJSON(w, http.StatusOK, c.Credential)
}
func (s *Server) VerifyCredentialHandler(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.VerifyCredential(r.PathValue("caseID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
