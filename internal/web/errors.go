package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"voice-clarity-acceptance/internal/domain"
)

type errorResponse struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Field    string `json:"field,omitempty"`
	Expected int64  `json:"expected,omitempty"`
	Actual   int64  `json:"actual,omitempty"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "invalid_json", Message: "JSON 请求无效: " + err.Error()})
		return false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "invalid_json", Message: "请求体只能包含一个 JSON 对象"})
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	resp := errorResponse{Code: code, Message: "服务发生内部错误"}
	var fieldErr *domain.FieldError
	var conflict *domain.ConflictError
	switch {
	case errors.As(err, &fieldErr):
		status = http.StatusUnprocessableEntity
		resp = errorResponse{Code: "validation_failed", Message: fieldErr.Message, Field: fieldErr.Field}
	case errors.As(err, &conflict):
		status = http.StatusConflict
		resp = errorResponse{Code: "version_conflict", Message: conflict.Error(), Expected: conflict.Expected, Actual: conflict.Actual}
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		resp = errorResponse{Code: "not_found", Message: "记录不存在"}
	case errors.Is(err, domain.ErrCredentialMissing):
		status = http.StatusNotFound
		resp = errorResponse{Code: "credential_missing", Message: err.Error()}
	case errors.Is(err, domain.ErrImmutable):
		status = http.StatusConflict
		resp = errorResponse{Code: "immutable", Message: err.Error()}
	case errors.Is(err, domain.ErrStateConflict):
		status = http.StatusConflict
		resp = errorResponse{Code: "state_conflict", Message: err.Error()}
	case errors.Is(err, domain.ErrVersionConflict):
		status = http.StatusConflict
		resp = errorResponse{Code: "version_conflict", Message: err.Error()}
	case errors.Is(err, domain.ErrAlreadyExists):
		status = http.StatusConflict
		resp = errorResponse{Code: "already_exists", Message: err.Error()}
	case errors.Is(err, domain.ErrEvidenceConflict):
		status = http.StatusConflict
		resp = errorResponse{Code: "evidence_conflict", Message: err.Error()}
	}
	writeJSON(w, status, resp)
}

type mutationMeta struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
}
