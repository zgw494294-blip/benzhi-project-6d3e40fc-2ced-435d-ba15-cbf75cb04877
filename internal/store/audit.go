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

// appendAudit appends a single audit event to the append-only log and returns
// the number of bytes written for the event so callers can roll back the
// append if a subsequent step fails. If encoding or syncing fails, any partial
// bytes written are truncated so the log remains parseable.
func appendAudit(path string, e AuditEvent) (int, error) {
	before, err := fileSize(path)
	if err != nil {
		return 0, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return 0, err
	}
	enc := json.NewEncoder(f)
	if err = enc.Encode(e); err != nil {
		f.Close()
		_ = os.Truncate(path, before)
		return 0, err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		_ = os.Truncate(path, before)
		return 0, err
	}
	if err = f.Close(); err != nil {
		_ = os.Truncate(path, before)
		return 0, err
	}
	after, err := fileSize(path)
	if err != nil {
		return 0, err
	}
	return int(after - before), nil
}

// rollbackAudit truncates the audit log back to the byte offset produced before
// the corresponding appendAudit call, restoring the previous consistent state.
func rollbackAudit(path string, written int) error {
	if written <= 0 {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	target := info.Size() - int64(written)
	if target < 0 {
		target = 0
	}
	return os.Truncate(path, target)
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	return info.Size(), nil
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
