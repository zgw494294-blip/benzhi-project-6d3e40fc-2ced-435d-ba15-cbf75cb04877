package store

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type auditMaterial struct {
	Sequence       uint64          `json:"sequence"`
	CaseID         string          `json:"caseID"`
	Type           string          `json:"type"`
	Actor          string          `json:"actor"`
	CaseVersion    int64           `json:"caseVersion"`
	OccurredAt     string          `json:"occurredAt"`
	Details        json.RawMessage `json:"details,omitempty"`
	PreviousDigest string          `json:"previousDigest"`
}

func eventDigest(e AuditEvent) (string, error) {
	m := auditMaterial{e.Sequence, e.CaseID, e.Type, e.Actor, e.CaseVersion, e.OccurredAt.UTC().Format("2006-01-02T15:04:05.000000000Z"), e.Details, e.PreviousDigest}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func appendAudit(f *os.File, e AuditEvent) error {
	enc := json.NewEncoder(f)
	if err := enc.Encode(e); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return nil
}

func readAudit(path string) ([]AuditEvent, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return []AuditEvent{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(bufio.NewReader(f))
	events := []AuditEvent{}
	var seq uint64
	prev := ""
	for {
		var e AuditEvent
		err = dec.Decode(&e)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("审计日志损坏: %w", err)
		}
		if e.Sequence != seq+1 {
			return nil, fmt.Errorf("审计序号不连续: %d", e.Sequence)
		}
		if e.PreviousDigest != prev {
			return nil, fmt.Errorf("审计前序摘要不匹配: %d", e.Sequence)
		}
		digest, digestErr := eventDigest(e)
		if digestErr != nil {
			return nil, digestErr
		}
		if digest != e.Digest {
			return nil, fmt.Errorf("审计摘要不匹配: %d", e.Sequence)
		}
		events = append(events, e)
		seq = e.Sequence
		prev = e.Digest
	}
	return events, nil
}
