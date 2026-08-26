package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"time"
	"voice-clarity-acceptance/internal/workflow"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	service    *workflow.Service
	mux        *http.ServeMux
	assetCache map[string][]byte
}

func New(service *workflow.Service) http.Handler {
	s := &Server{service: service, mux: http.NewServeMux(), assetCache: make(map[string][]byte)}
	s.routes()
	return s.recoverPanic(s.securityHeaders(s.requestLimit(s.mux)))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.HealthHandler)
	s.mux.HandleFunc("GET /api/cases", s.ListCasesHandler)
	s.mux.HandleFunc("POST /api/cases", s.CreateCaseHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}", s.GetCaseHandler)
	s.mux.HandleFunc("PUT /api/cases/{caseID}/scope", s.UpdateScopeHandler)
	s.mux.HandleFunc("PUT /api/cases/{caseID}/plan", s.SavePlanHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/plan/freeze", s.FreezePlanHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/plan/precheck", s.PrecheckPlanHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}/plan/revisions", s.PlanRevisionsHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}/plan/revisions/compare", s.ComparePlanRevisionsHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/plan/revisions/{revision}/restore", s.RestorePlanHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/rounds/{roundID}/readings", s.AddReadingHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/rounds/{roundID}/readings/{readingID}/correct", s.CorrectReadingHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/rounds/{roundID}/readings/{readingID}/corrections", s.CorrectReadingHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/rounds/{roundID}/readings/correct", s.CorrectReadingHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/rounds/{roundID}/close", s.CloseRoundHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}/rounds/{roundID}/quality-gate", s.QualityGateHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}/quality-gate", s.CaseQualityGateHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}/decision/review", s.DecisionReviewHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/deviations", s.CreateDeviationHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/deviations/{deviationID}/executions", s.AddExecutionHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}/deviations/candidates", s.RetestCandidatesHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/review", s.ReviewHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}/review/gate", s.ApprovalGateHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/review/issues/{issueID}/resolve", s.ResolveIssueHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}/audit", s.AuditHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}/credential", s.CredentialHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/credential/verify", s.VerifyCredentialHandler)
	s.mux.HandleFunc("GET /api/credentials", s.LookupCredentialHandler)
	s.mux.HandleFunc("GET /api/credentials/lookup", s.LookupCredentialHandler)
	s.mux.HandleFunc("GET /api/credentials/{credentialID}", s.LookupCredentialPathHandler)
	s.mux.HandleFunc("POST /api/credentials/{credentialID}/verify", s.VerifyCredentialLookupHandler)
	assets, _ := fs.Sub(staticFiles, "static")
	s.mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
	s.mux.HandleFunc("GET /", s.WorkbenchHandler)
}

func (s *Server) WorkbenchHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	const asset = "static/index.html"
	b, ok := s.assetCache[asset]
	if !ok {
		var err error
		b, err = staticFiles.ReadFile(asset)
		if err != nil {
			writeError(w, err)
			return
		}
		s.assetCache[asset] = b
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(b)
}
func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
func (s *Server) requestLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal_error", Message: "服务发生内部错误"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func actor(r *http.Request) string {
	v := strings.TrimSpace(r.Header.Get("X-Actor"))
	if v == "" {
		return "浏览器用户"
	}
	return v
}
