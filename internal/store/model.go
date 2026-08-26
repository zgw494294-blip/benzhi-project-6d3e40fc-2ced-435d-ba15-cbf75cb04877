package store

import (
	"encoding/json"
	"time"
	"voice-clarity-acceptance/internal/domain"
)

const SchemaVersion = 1

type Snapshot struct {
	SchemaVersion       int                               `json:"schemaVersion"`
	Cases               map[string]*domain.AcceptanceCase `json:"cases"`
	CaseNumbers         map[string]string                 `json:"caseNumbers"`
	Idempotency         map[string]IdempotencyResult      `json:"idempotency"`
	CredentialSequence  uint64                            `json:"credentialSequence"`
	CredentialIDs       map[string]string                 `json:"credentialIDs"`
	CredentialSequences map[uint64]string                 `json:"credentialSequences"`
	AuditSequence       uint64                            `json:"auditSequence"`
	LastAuditDigest     string                            `json:"lastAuditDigest"`
	SavedAt             time.Time                         `json:"savedAt"`
}

type IdempotencyResult struct {
	CaseID      string          `json:"caseID"`
	Operation   string          `json:"operation"`
	Version     int64           `json:"version"`
	Response    json.RawMessage `json:"response,omitempty"`
	CommittedAt time.Time       `json:"committedAt"`
}

type AuditEvent struct {
	Sequence       uint64          `json:"sequence"`
	CaseID         string          `json:"caseID"`
	Type           string          `json:"type"`
	Actor          string          `json:"actor"`
	CaseVersion    int64           `json:"caseVersion"`
	OccurredAt     time.Time       `json:"occurredAt"`
	Details        json.RawMessage `json:"details,omitempty"`
	PreviousDigest string          `json:"previousDigest"`
	Digest         string          `json:"digest"`
}

func newSnapshot() Snapshot {
	return Snapshot{SchemaVersion: SchemaVersion, Cases: map[string]*domain.AcceptanceCase{}, CaseNumbers: map[string]string{}, Idempotency: map[string]IdempotencyResult{}, CredentialIDs: map[string]string{}, CredentialSequences: map[uint64]string{}}
}

func cloneCase(c *domain.AcceptanceCase) (*domain.AcceptanceCase, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var out domain.AcceptanceCase
	if err = json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
