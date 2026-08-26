package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"voice-clarity-acceptance/internal/domain"
)

type ReviewCommand struct {
	CaseID, Decision, Reviewer, Comment string
	ExpectedVersion                     int64
	IdempotencyKey                      string
	Issues                              []domain.ReviewIssue
}

func (s *Service) Review(cmd ReviewCommand) (*domain.AcceptanceCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok, err := s.existing(cmd.IdempotencyKey, "review"); ok || err != nil {
		return c, err
	}
	c, err := s.loadForWrite(cmd.CaseID, cmd.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	reviewer := strings.TrimSpace(cmd.Reviewer)
	if reviewer == "" {
		return nil, &domain.FieldError{Field: "reviewer", Message: "不能为空"}
	}
	decision := strings.ToLower(strings.TrimSpace(cmd.Decision))
	if decision != "approve" && decision != "return" {
		return nil, &domain.FieldError{Field: "decision", Message: "仅支持 approve 或 return"}
	}
	if decision == "return" && strings.TrimSpace(cmd.Comment) == "" {
		return nil, &domain.FieldError{Field: "comment", Message: "退回必须填写意见"}
	}
	if decision == "return" {
		if len(cmd.Issues) == 0 {
			return nil, &domain.FieldError{Field: "issues", Message: "退回至少登记一个事项"}
		}
		seen := map[string]bool{}
		for i := range cmd.Issues {
			issue := &cmd.Issues[i]
			if strings.TrimSpace(issue.Category) == "" || strings.TrimSpace(issue.Description) == "" || strings.TrimSpace(issue.Requirement) == "" {
				return nil, &domain.FieldError{Field: fmt.Sprintf("issues[%d]", i), Message: "类别、说明和整改要求不能为空"}
			}
			key := strings.ToLower(strings.TrimSpace(issue.Category)) + "|" + issue.ZoneID + "|" + issue.PointID + "|" + issue.RoundID
			if seen[key] {
				return nil, &domain.FieldError{Field: fmt.Sprintf("issues[%d]", i), Message: "退回事项不能重复"}
			}
			seen[key] = true
			if issue.ZoneID != "" {
				if _, ok := c.FindZone(issue.ZoneID); !ok {
					return nil, &domain.FieldError{Field: fmt.Sprintf("issues[%d].zoneID", i), Message: "区域不属于当前验收案"}
				}
			}
			if issue.PointID != "" {
				p, ok := c.FindPoint(issue.PointID)
				if !ok {
					return nil, &domain.FieldError{Field: fmt.Sprintf("issues[%d].pointID", i), Message: "测点不属于当前验收案"}
				}
				if issue.ZoneID != "" && p.ZoneID != issue.ZoneID {
					return nil, &domain.FieldError{Field: fmt.Sprintf("issues[%d].pointID", i), Message: "测点不属于引用区域"}
				}
			}
			if issue.RoundID != "" {
				if _, ok := c.FindRound(issue.RoundID); !ok {
					return nil, &domain.FieldError{Field: fmt.Sprintf("issues[%d].roundID", i), Message: "轮次不属于当前验收案"}
				}
			}
			issue.ID = newID("issue")
			issue.Description = strings.TrimSpace(issue.Description)
			issue.Requirement = strings.TrimSpace(issue.Requirement)
			issue.Category = strings.TrimSpace(issue.Category)
		}
	}
	now := s.now()
	record := domain.ReviewRecord{ID: newID("review"), Decision: decision, Reviewer: reviewer, Comment: strings.TrimSpace(cmd.Comment), CaseVersion: c.Version, ReviewedAt: now, Issues: append([]domain.ReviewIssue(nil), cmd.Issues...)}
	var credential *domain.AcceptanceCredential
	if decision == "approve" {
		if err = approvalGate(c); err != nil {
			return nil, err
		}
		digest, err := domain.SnapshotDigest(c)
		if err != nil {
			return nil, err
		}
		seq := s.nextCredentialSequence
		material := sha256.Sum256([]byte(c.ID + digest + reviewer))
		credential = &domain.AcceptanceCredential{ID: "credential_" + hex.EncodeToString(material[:8]), CaseID: c.ID, CaseVersion: c.Version + 1, Decision: "approved", Reviewer: reviewer, IssuedAt: now, Sequence: seq, SnapshotDigest: digest, PreviousAuditDigest: s.store.LastAuditDigest()}
	}
	if err = c.Review(record, credential, now); err != nil {
		return nil, err
	}
	event := "review.returned"
	if decision == "approve" {
		event = "credential.issued"
	}
	if err = commit(s, c, "review", cmd.IdempotencyKey, reviewer, event, map[string]any{"decision": decision, "credential": credential, "issueCount": len(record.Issues), "issues": record.Issues}); err != nil {
		return nil, err
	}
	if credential != nil {
		s.nextCredentialSequence++
	}
	return c, nil
}

func approvalGate(c *domain.AcceptanceCase) error {
	if c.Decision == nil || !c.Decision.Passed {
		return &domain.FieldError{Field: "decision", Message: "整案判定尚未通过"}
	}
	if len(c.Decision.MissingPointIDs) > 0 {
		return &domain.FieldError{Field: "decision", Message: "仍有缺失测点"}
	}
	if len(c.Decision.InvalidPointIDs) > 0 {
		return &domain.FieldError{Field: "decision", Message: "仍有无效读数"}
	}
	for _, d := range c.Deviations {
		if d.Status == domain.DeviationOpen {
			return &domain.FieldError{Field: "deviations", Message: "仍有未关闭偏差"}
		}
	}
	for _, r := range c.Rounds {
		if r.Status == domain.RoundOpen {
			return &domain.FieldError{Field: "rounds", Message: "仍有开放的复测轮次"}
		}
	}
	for _, review := range c.Reviews {
		for _, issue := range review.Issues {
			if !issue.Resolved {
				return &domain.FieldError{Field: "reviewIssues", Message: "仍有未解决的退回事项"}
			}
		}
	}
	return nil
}

type CredentialVerification struct {
	Valid                   bool   `json:"valid"`
	CredentialID            string `json:"credentialID"`
	Sequence                uint64 `json:"sequence"`
	StoredDigest            string `json:"storedDigest"`
	RecalculatedDigest      string `json:"recalculatedDigest"`
	AuditValid              bool   `json:"auditValid"`
	Message                 string `json:"message"`
	AuditEventCount         int    `json:"auditEventCount"`
	AuditFirstErrorSequence uint64 `json:"auditFirstErrorSequence,omitempty"`
	AuditErrorType          string `json:"auditErrorType,omitempty"`
	AnchorValid             bool   `json:"anchorValid"`
	CaseVersionValid        bool   `json:"caseVersionValid"`
}

func (s *Service) VerifyCredential(caseID string) (CredentialVerification, error) {
	c, err := s.store.GetCase(caseID)
	if err != nil {
		return CredentialVerification{}, err
	}
	if c.Credential == nil {
		return CredentialVerification{}, domain.ErrCredentialMissing
	}
	digest, err := domain.SnapshotDigestForCredential(c)
	if err != nil {
		return CredentialVerification{}, err
	}
	diagnostic := s.store.DiagnoseAudit(caseID)
	auditErr := s.store.VerifyAudit()
	anchorValid := false
	for _, e := range s.store.AuditForCase(caseID) {
		if e.Type == "credential.issued" && e.CaseVersion == c.Credential.CaseVersion {
			anchorValid = true
			break
		}
	}
	caseVersionValid := c.Credential.CaseVersion == c.Version && c.Credential.Decision == "approved" && c.Status == domain.StatusApproved
	valid := digest == c.Credential.SnapshotDigest && auditErr == nil && anchorValid && caseVersionValid
	message := "凭据摘要与审计链校验通过"
	if !valid {
		message = "凭据摘要或审计链校验失败"
	}
	return CredentialVerification{Valid: valid, CredentialID: c.Credential.ID, Sequence: c.Credential.Sequence, StoredDigest: c.Credential.SnapshotDigest, RecalculatedDigest: digest, AuditValid: diagnostic.Valid, Message: message, AuditEventCount: diagnostic.EventCount, AuditFirstErrorSequence: diagnostic.FirstErrorSequence, AuditErrorType: diagnostic.ErrorType, AnchorValid: anchorValid, CaseVersionValid: caseVersionValid}, nil
}
