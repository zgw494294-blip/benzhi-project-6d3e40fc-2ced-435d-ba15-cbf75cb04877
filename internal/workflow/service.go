package workflow

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
	"voice-clarity-acceptance/internal/domain"
	"voice-clarity-acceptance/internal/store"
)

type Clock func() time.Time

type Service struct {
	mu    sync.Mutex
	store *store.Store
	now   Clock
}

func New(s *store.Store) *Service {
	return &Service{store: s, now: func() time.Time { return time.Now().UTC() }}
}

func newID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(b)
}

func (s *Service) GetCase(id string) (*domain.AcceptanceCase, error) { return s.store.GetCase(id) }
func (s *Service) ListCases() ([]*domain.AcceptanceCase, error)      { return s.store.ListCases() }
func (s *Service) Audit(id string) []store.AuditEvent                { return s.store.AuditForCase(id) }

func (s *Service) existing(key, operation string) (*domain.AcceptanceCase, bool, error) {
	return s.existingForCase("", key, operation)
}
func (s *Service) existingForCase(caseID, key, operation string) (*domain.AcceptanceCase, bool, error) {
	if key == "" {
		return nil, false, &domain.FieldError{Field: "idempotencyKey", Message: "不能为空"}
	}
	var prior store.IdempotencyResult
	var ok bool
	if caseID == "" {
		prior, ok = s.store.FindIdempotency(key)
	} else {
		prior, ok = s.store.FindIdempotencyForCase(caseID, key)
	}
	if !ok {
		return nil, false, nil
	}
	if prior.Operation != operation {
		return nil, false, &domain.FieldError{Field: "idempotencyKey", Message: "已用于其他操作"}
	}
	c, err := s.store.GetCase(prior.CaseID)
	return c, true, err
}

func (s *Service) loadForWrite(caseID string, expected int64) (*domain.AcceptanceCase, error) {
	c, err := s.store.GetCase(caseID)
	if err != nil {
		return nil, err
	}
	if err = c.EnsureMutable(); err != nil {
		return nil, err
	}
	if err = c.CheckVersion(expected); err != nil {
		return nil, err
	}
	return c, nil
}

func commit(s *Service, c *domain.AcceptanceCase, operation, key, actor, event string, details any) error {
	_, err := s.store.Commit(store.CommitRequest{Case: c, Operation: operation, IdempotencyKey: key, Actor: actor, EventType: event, Response: map[string]any{"caseID": c.ID, "version": c.Version}, Details: details})
	return err
}
