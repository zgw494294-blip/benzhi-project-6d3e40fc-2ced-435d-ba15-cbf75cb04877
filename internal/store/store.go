package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
	"voice-clarity-acceptance/internal/domain"
)

type Store struct {
	mu                           sync.RWMutex
	dir, snapshotPath, auditPath string
	auditFile                    *os.File
	state                        Snapshot
	audit                        []AuditEvent
}

type CommitRequest struct {
	Case                                        *domain.AcceptanceCase
	Operation, IdempotencyKey, Actor, EventType string
	Response                                    any
	Details                                     any
}

func Open(dir string) (*Store, error) {
	if dir == "" {
		return nil, fmt.Errorf("数据目录不能为空")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	snapshotPath := filepath.Join(dir, "snapshot.json")
	auditPath := filepath.Join(dir, "audit.jsonl")
	s, err := loadSnapshot(snapshotPath)
	if err != nil {
		return nil, err
	}
	events, err := readAudit(auditPath)
	if err != nil {
		return nil, err
	}
	var seq uint64
	digest := ""
	if len(events) > 0 {
		last := events[len(events)-1]
		seq, digest = last.Sequence, last.Digest
	}
	if s.AuditSequence != seq || s.LastAuditDigest != digest {
		return nil, fmt.Errorf("快照与审计日志不一致")
	}
	if err := validateCredentialIndexes(s); err != nil {
		return nil, err
	}
	auditFile, err := os.OpenFile(auditPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	return &Store{dir: dir, snapshotPath: snapshotPath, auditPath: auditPath, auditFile: auditFile, state: s, audit: events}, nil
}

func (s *Store) GetCase(id string) (*domain.AcceptanceCase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.state.Cases[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return cloneCase(c)
}
func (s *Store) ListCases() ([]*domain.AcceptanceCase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.AcceptanceCase, 0, len(s.state.Cases))
	for _, c := range s.state.Cases {
		x, err := cloneCase(c)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) FindIdempotency(key string) (IdempotencyResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.state.Idempotency[key]
	return r, ok
}
func (s *Store) CaseNumberExists(number string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.state.CaseNumbers[number]
	return ok
}
func (s *Store) LastAuditDigest() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.LastAuditDigest
}
func (s *Store) NextCredentialSequence() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.CredentialSequence + 1
}
func (s *Store) FindCredential(id string, sequence uint64) (*domain.AcceptanceCase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	caseID := ""
	if id != "" {
		caseID = s.state.CredentialIDs[id]
	} else if sequence > 0 {
		caseID = s.state.CredentialSequences[sequence]
	}
	c, ok := s.state.Cases[caseID]
	if !ok || c.Credential == nil {
		return nil, domain.ErrNotFound
	}
	return cloneCase(c)
}

func (s *Store) Commit(req CommitRequest) (IdempotencyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.IdempotencyKey == "" {
		return IdempotencyResult{}, &domain.FieldError{Field: "idempotencyKey", Message: "不能为空"}
	}
	if prior, ok := s.state.Idempotency[req.IdempotencyKey]; ok {
		return prior, nil
	}
	if old, ok := s.state.Cases[req.Case.ID]; ok && old.Version >= req.Case.Version {
		return IdempotencyResult{}, domain.ErrVersionConflict
	}
	caseCopy, err := cloneCase(req.Case)
	if err != nil {
		return IdempotencyResult{}, err
	}
	response, err := json.Marshal(req.Response)
	if err != nil {
		return IdempotencyResult{}, err
	}
	details, err := json.Marshal(req.Details)
	if err != nil {
		return IdempotencyResult{}, err
	}
	now := time.Now().UTC()
	event := AuditEvent{Sequence: s.state.AuditSequence + 1, CaseID: req.Case.ID, Type: req.EventType, Actor: req.Actor, CaseVersion: req.Case.Version, OccurredAt: now, Details: details, PreviousDigest: s.state.LastAuditDigest}
	event.Digest, err = eventDigest(event)
	if err != nil {
		return IdempotencyResult{}, err
	}
	next := s.state
	next.Cases = copyCases(s.state.Cases)
	next.CaseNumbers = copyStrings(s.state.CaseNumbers)
	next.Idempotency = copyIdempotency(s.state.Idempotency)
	next.CredentialIDs = copyStrings(s.state.CredentialIDs)
	next.CredentialSequences = copySequences(s.state.CredentialSequences)
	next.Cases[req.Case.ID] = caseCopy
	next.CaseNumbers[req.Case.CaseNumber] = req.Case.ID
	result := IdempotencyResult{CaseID: req.Case.ID, Operation: req.Operation, Version: req.Case.Version, Response: response, CommittedAt: now}
	next.Idempotency[req.IdempotencyKey] = result
	next.AuditSequence = event.Sequence
	next.LastAuditDigest = event.Digest
	next.SavedAt = now
	if req.Case.Credential != nil && req.Case.Credential.Sequence > next.CredentialSequence {
		next.CredentialSequence = req.Case.Credential.Sequence
		if owner, ok := next.CredentialIDs[req.Case.Credential.ID]; ok && owner != req.Case.ID {
			return IdempotencyResult{}, fmt.Errorf("凭据 ID 重复")
		}
		if owner, ok := next.CredentialSequences[req.Case.Credential.Sequence]; ok && owner != req.Case.ID {
			return IdempotencyResult{}, fmt.Errorf("凭据序号重复")
		}
		next.CredentialIDs[req.Case.Credential.ID] = req.Case.ID
		next.CredentialSequences[req.Case.Credential.Sequence] = req.Case.ID
	}
	if err = appendAudit(s.auditFile, event); err != nil {
		return IdempotencyResult{}, err
	}
	if err = writeSnapshot(s.snapshotPath, next); err != nil {
		return IdempotencyResult{}, fmt.Errorf("审计已追加但快照提交失败: %w", err)
	}
	s.state = next
	s.audit = append(s.audit, event)
	return result, nil
}
func copySequences(in map[uint64]string) map[uint64]string {
	out := make(map[uint64]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func validateCredentialIndexes(s Snapshot) error {
	seenID := map[string]string{}
	seenSeq := map[uint64]string{}
	var max uint64
	for id, c := range s.Cases {
		if c.Credential == nil {
			continue
		}
		cr := c.Credential
		if cr.CaseID != id {
			return fmt.Errorf("凭据与验收案归属不一致")
		}
		if _, ok := seenID[cr.ID]; ok {
			return fmt.Errorf("凭据 ID 重复")
		}
		if _, ok := seenSeq[cr.Sequence]; ok {
			return fmt.Errorf("凭据序号重复")
		}
		seenID[cr.ID] = id
		seenSeq[cr.Sequence] = id
		if cr.Sequence > max {
			max = cr.Sequence
		}
	}
	if max != s.CredentialSequence {
		return fmt.Errorf("全局凭据序号不一致")
	}
	for n := uint64(1); n <= max; n++ {
		if _, ok := seenSeq[n]; !ok {
			return fmt.Errorf("凭据序号不连续")
		}
	}
	if len(seenID) != len(s.CredentialIDs) || len(seenSeq) != len(s.CredentialSequences) {
		return fmt.Errorf("凭据索引与案卷不一致")
	}
	for k, v := range s.CredentialIDs {
		if seenID[k] != v {
			return fmt.Errorf("凭据 ID 索引悬空或归属错误")
		}
	}
	for k, v := range s.CredentialSequences {
		if seenSeq[k] != v {
			return fmt.Errorf("凭据序号索引悬空或归属错误")
		}
	}
	return nil
}

func copyCases(in map[string]*domain.AcceptanceCase) map[string]*domain.AcceptanceCase {
	out := make(map[string]*domain.AcceptanceCase, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func copyStrings(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func copyIdempotency(in map[string]IdempotencyResult) map[string]IdempotencyResult {
	out := make(map[string]IdempotencyResult, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (s *Store) AuditForCase(caseID string) []AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []AuditEvent{}
	for _, e := range s.audit {
		if e.CaseID == caseID {
			out = append(out, e)
		}
	}
	return out
}
func (s *Store) VerifyAudit() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var seq uint64
	prev := ""
	for _, e := range s.audit {
		if e.Sequence != seq+1 || e.PreviousDigest != prev {
			return fmt.Errorf("审计链断裂")
		}
		d, err := eventDigest(e)
		if err != nil || d != e.Digest {
			return fmt.Errorf("审计摘要无效")
		}
		seq = e.Sequence
		prev = e.Digest
	}
	return nil
}

type AuditDiagnostic struct {
	Valid              bool   `json:"valid"`
	EventCount         int    `json:"eventCount"`
	FirstErrorSequence uint64 `json:"firstErrorSequence,omitempty"`
	ErrorType          string `json:"errorType,omitempty"`
}

func (s *Store) DiagnoseAudit(caseID string) AuditDiagnostic {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := s.audit
	var seq uint64
	prev := ""
	for _, e := range all {
		if e.Sequence != seq+1 {
			return AuditDiagnostic{Valid: false, EventCount: countCaseAudit(all, caseID), FirstErrorSequence: e.Sequence, ErrorType: "sequence"}
		}
		if e.PreviousDigest != prev {
			return AuditDiagnostic{Valid: false, EventCount: countCaseAudit(all, caseID), FirstErrorSequence: e.Sequence, ErrorType: "previous_digest"}
		}
		d, err := eventDigest(e)
		if err != nil || d != e.Digest {
			return AuditDiagnostic{Valid: false, EventCount: countCaseAudit(all, caseID), FirstErrorSequence: e.Sequence, ErrorType: "event_digest"}
		}
		seq = e.Sequence
		prev = e.Digest
	}
	return AuditDiagnostic{Valid: true, EventCount: countCaseAudit(all, caseID)}
}

func countCaseAudit(events []AuditEvent, caseID string) int {
	n := 0
	for _, e := range events {
		if e.CaseID == caseID {
			n++
		}
	}
	return n
}
