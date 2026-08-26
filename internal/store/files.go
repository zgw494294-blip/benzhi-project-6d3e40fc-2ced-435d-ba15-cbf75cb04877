package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"voice-clarity-acceptance/internal/domain"
)

func loadSnapshot(path string) (Snapshot, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return newSnapshot(), nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	var s Snapshot
	if err = json.Unmarshal(b, &s); err != nil {
		return Snapshot{}, fmt.Errorf("快照解析失败: %w", err)
	}
	if s.SchemaVersion != SchemaVersion {
		return Snapshot{}, fmt.Errorf("不支持的 schemaVersion: %d", s.SchemaVersion)
	}
	return normalizeSnapshot(s), nil
}

func normalizeSnapshot(s Snapshot) Snapshot {
	if s.Cases == nil {
		s.Cases = make(map[string]*domain.AcceptanceCase)
	}
	if s.CaseNumbers == nil {
		s.CaseNumbers = map[string]string{}
	}
	if s.Idempotency == nil {
		s.Idempotency = map[string]IdempotencyResult{}
	}
	if s.CredentialIDs == nil {
		s.CredentialIDs = map[string]string{}
		for id, c := range s.Cases {
			if c.Credential != nil {
				s.CredentialIDs[c.Credential.ID] = id
			}
		}
	}
	if s.CredentialSequences == nil {
		s.CredentialSequences = map[uint64]string{}
		for id, c := range s.Cases {
			if c.Credential != nil {
				s.CredentialSequences[c.Credential.Sequence] = id
			}
		}
	}
	return s
}

func writeSnapshot(path string, s Snapshot) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "snapshot-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	ok := false
	defer func() {
		tmp.Close()
		if !ok {
			os.Remove(name)
		}
	}()
	if err = tmp.Chmod(0600); err != nil {
		return err
	}
	if _, err = tmp.Write(b); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err = d.Sync(); err != nil {
		d.Close()
		return err
	}
	if err = d.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}
