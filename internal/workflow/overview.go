package workflow

import (
	"sort"
	"strings"
	"time"
	"voice-clarity-acceptance/internal/domain"
)

type CaseQuery struct {
	Keyword, Status, Sort string
	Page, PageSize        int
}
type CaseSummary struct {
	ID                     string            `json:"id"`
	CaseNumber             string            `json:"caseNumber"`
	SiteName               string            `json:"siteName"`
	ResponsibleEngineer    string            `json:"responsibleEngineer"`
	Status                 domain.CaseStatus `json:"status"`
	ZoneCount              int               `json:"zoneCount"`
	PointCount             int               `json:"pointCount"`
	PlannedPointCount      int               `json:"plannedPointCount"`
	LatestRound            int               `json:"latestRound"`
	LatestMeasurementRound int               `json:"latestMeasurementRound"`
	OpenDeviationCount     int               `json:"openDeviationCount"`
	Version                int64             `json:"version"`
	UpdatedAt              time.Time         `json:"updatedAt"`
	CreatedAt              time.Time         `json:"createdAt"`
}
type CasePage struct {
	Items      []CaseSummary `json:"items"`
	Total      int           `json:"total"`
	TotalCount int           `json:"totalCount"`
	Page       int           `json:"page"`
	PageSize   int           `json:"pageSize"`
	TotalPages int           `json:"totalPages"`
}

func (s *Service) QueryCases(q CaseQuery) (CasePage, error) {
	if q.Page < 1 {
		return CasePage{}, &domain.FieldError{Field: "page", Message: "必须是正整数"}
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		return CasePage{}, &domain.FieldError{Field: "pageSize", Message: "必须在 1 至 100 之间"}
	}
	if q.Sort != "" && q.Sort != "updatedAt" && q.Sort != "createdAt" && q.Sort != "caseNumber" {
		return CasePage{}, &domain.FieldError{Field: "sort", Message: "仅支持 updatedAt、createdAt 或 caseNumber"}
	}
	allowed := map[domain.CaseStatus]bool{domain.StatusDraft: true, domain.StatusPlanFrozen: true, domain.StatusMeasuring: true, domain.StatusRemediation: true, domain.StatusPendingReview: true, domain.StatusReturned: true, domain.StatusApproved: true}
	if q.Status != "" && !allowed[domain.CaseStatus(q.Status)] {
		return CasePage{}, &domain.FieldError{Field: "status", Message: "未知验收案状态"}
	}
	cs, e := s.store.ListCases()
	if e != nil {
		return CasePage{}, e
	}
	k := strings.ToLower(strings.TrimSpace(q.Keyword))
	items := []CaseSummary{}
	for _, c := range cs {
		if q.Status != "" && string(c.Status) != q.Status {
			continue
		}
		if k != "" && !strings.Contains(strings.ToLower(c.CaseNumber), k) && !strings.Contains(strings.ToLower(c.SiteName), k) && !strings.Contains(strings.ToLower(c.ResponsibleEngineer), k) {
			continue
		}
		latest := 0
		for _, r := range c.Rounds {
			if r.Number > latest {
				latest = r.Number
			}
		}
		open := 0
		for _, d := range c.Deviations {
			if d.Status == domain.DeviationOpen {
				open++
			}
		}
		items = append(items, CaseSummary{ID: c.ID, CaseNumber: c.CaseNumber, SiteName: c.SiteName, ResponsibleEngineer: c.ResponsibleEngineer, Status: c.Status, ZoneCount: len(c.Zones), PointCount: len(c.Points), PlannedPointCount: len(c.Points), LatestRound: latest, LatestMeasurementRound: latest, OpenDeviationCount: open, Version: c.Version, UpdatedAt: c.UpdatedAt, CreatedAt: c.CreatedAt})
	}
	sort.SliceStable(items, func(i, j int) bool {
		switch q.Sort {
		case "createdAt":
			if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
				return items[i].CreatedAt.After(items[j].CreatedAt)
			}
		case "caseNumber":
			if items[i].CaseNumber != items[j].CaseNumber {
				return items[i].CaseNumber < items[j].CaseNumber
			}
		default:
			if !items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
				return items[i].UpdatedAt.After(items[j].UpdatedAt)
			}
		}
		return items[i].ID < items[j].ID
	})
	total := len(items)
	start := (q.Page - 1) * q.PageSize
	if start > total {
		start = total
	}
	end := start + q.PageSize
	if end > total {
		end = total
	}
	pages := (total + q.PageSize - 1) / q.PageSize
	return CasePage{Items: items[start:end], Total: total, TotalCount: total, Page: q.Page, PageSize: q.PageSize, TotalPages: pages}, nil
}
