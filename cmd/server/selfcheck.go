package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"voice-clarity-acceptance/internal/domain"
)

type selfcheckClient struct {
	base   string
	client *http.Client
}

func runSelfcheck(base string) error {
	c := &selfcheckClient{base: base, client: &http.Client{Timeout: 5 * time.Second}}
	var health map[string]any
	if err := c.call(http.MethodGet, "/api/health", nil, &health); err != nil {
		return fmt.Errorf("健康检查失败: %w", err)
	}
	create := map[string]any{
		"caseNumber":          "SELF-CHECK-001",
		"siteName":            "自检公共建筑",
		"responsibleEngineer": "自检工程师",
		"expectedVersion":     0,
		"idempotencyKey":      "self-create",
		"zones": []map[string]any{{
			"id": "zone-self", "name": "自检大厅", "usageClass": "公共大厅",
			"areaSquareMeters": 160, "minimumPointCount": 2, "intelligibilityThreshold": 0.60,
		}},
	}
	var acceptance domain.AcceptanceCase
	if err := c.call(http.MethodPost, "/api/cases", create, &acceptance); err != nil {
		return fmt.Errorf("创建验收案失败: %w", err)
	}
	plan := map[string]any{"expectedVersion": acceptance.Version, "idempotencyKey": "self-plan", "points": []map[string]any{
		{"id": "point-self-1", "zoneID": "zone-self", "pointCode": "P-01", "locationDescription": "大厅北侧", "coverageTag": "north"},
		{"id": "point-self-2", "zoneID": "zone-self", "pointCode": "P-02", "locationDescription": "大厅南侧", "coverageTag": "south"},
	}}
	if err := c.call(http.MethodPut, "/api/cases/"+acceptance.ID+"/plan", plan, &acceptance); err != nil {
		return fmt.Errorf("保存测点计划失败: %w", err)
	}
	var precheck struct {
		Revision         int    `json:"revision"`
		Version          int64  `json:"version"`
		CandidateSummary string `json:"candidateSummary"`
		Freezable        bool   `json:"freezable"`
	}
	if err := c.call(http.MethodPost, "/api/cases/"+acceptance.ID+"/plan/precheck", map[string]any{}, &precheck); err != nil {
		return fmt.Errorf("计划预检失败: %w", err)
	}
	if !precheck.Freezable {
		return fmt.Errorf("计划预检未通过")
	}
	if err := c.call(http.MethodPost, "/api/cases/"+acceptance.ID+"/plan/freeze", map[string]any{"expectedVersion": acceptance.Version, "idempotencyKey": "self-freeze", "planRevision": precheck.Revision, "candidateSummary": precheck.CandidateSummary}, &acceptance); err != nil {
		return fmt.Errorf("冻结测点计划失败: %w", err)
	}
	roundID := acceptance.Rounds[0].ID
	body := map[string]any{"expectedVersion": acceptance.Version, "idempotencyKey": "self-readings-batch", "readings": []map[string]any{
		{"pointID": "point-self-1", "backgroundNoiseDBA": 44.5, "broadcastLevelDBA": 72.0, "intelligibilityValue": 0.76, "instrumentID": "SELF-STIPA-01", "measuredAt": time.Now().UTC()},
		{"pointID": "point-self-2", "backgroundNoiseDBA": 44.5, "broadcastLevelDBA": 72.0, "intelligibilityValue": 0.76, "instrumentID": "SELF-STIPA-01", "measuredAt": time.Now().UTC()},
	}}
	if err := c.call(http.MethodPost, "/api/cases/"+acceptance.ID+"/rounds/"+roundID+"/readings", body, &acceptance); err != nil {
		return fmt.Errorf("批量提交读数失败: %w", err)
	}
	if err := c.call(http.MethodPost, "/api/cases/"+acceptance.ID+"/rounds/"+roundID+"/close", map[string]any{"expectedVersion": acceptance.Version, "idempotencyKey": "self-close"}, &acceptance); err != nil {
		return fmt.Errorf("关闭轮次失败: %w", err)
	}
	if acceptance.Decision == nil || !acceptance.Decision.Passed {
		return fmt.Errorf("自检整案判定未通过")
	}
	review := map[string]any{"expectedVersion": acceptance.Version, "idempotencyKey": "self-review", "decision": "approve", "reviewer": "自检审核员", "comment": "自检通过"}
	if err := c.call(http.MethodPost, "/api/cases/"+acceptance.ID+"/review", review, &acceptance); err != nil {
		return fmt.Errorf("审核批准失败: %w", err)
	}
	var verification struct {
		Valid bool `json:"valid"`
	}
	if err := c.call(http.MethodPost, "/api/cases/"+acceptance.ID+"/credential/verify", map[string]any{}, &verification); err != nil {
		return fmt.Errorf("凭据校验请求失败: %w", err)
	}
	if !verification.Valid {
		return fmt.Errorf("凭据摘要校验未通过")
	}
	var audit struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := c.call(http.MethodGet, "/api/cases/"+acceptance.ID+"/audit", nil, &audit); err != nil {
		return fmt.Errorf("审计查询失败: %w", err)
	}
	if len(audit.Items) < 6 {
		return fmt.Errorf("审计事件数量不足: %d", len(audit.Items))
	}
	return nil
}

func (c *selfcheckClient) call(method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Actor", "自检客户端")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}
