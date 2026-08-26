package domain

import "time"

type CaseStatus string

const (
	StatusDraft         CaseStatus = "draft"
	StatusPlanFrozen    CaseStatus = "plan_frozen"
	StatusMeasuring     CaseStatus = "measuring"
	StatusRemediation   CaseStatus = "remediation"
	StatusPendingReview CaseStatus = "pending_review"
	StatusReturned      CaseStatus = "returned"
	StatusApproved      CaseStatus = "approved"
)

type RoundStatus string

const (
	RoundOpen   RoundStatus = "open"
	RoundClosed RoundStatus = "closed"
)

type DeviationStatus string

const (
	DeviationOpen   DeviationStatus = "open"
	DeviationClosed DeviationStatus = "closed"
)

type ValidityStatus string

const (
	ReadingValid   ValidityStatus = "valid"
	ReadingInvalid ValidityStatus = "invalid"
)

type PointDecision struct {
	PointID        string    `json:"pointID"`
	RoundID        string    `json:"roundID"`
	Valid          bool      `json:"valid"`
	Passed         bool      `json:"passed"`
	Value          float64   `json:"value"`
	Threshold      float64   `json:"threshold"`
	Reason         string    `json:"reason,omitempty"`
	Difference     float64   `json:"difference,omitempty"`
	MeasuredAt     time.Time `json:"measuredAt,omitempty"`
	ValidityReason string    `json:"validityReason,omitempty"`
	SourceKind     string    `json:"sourceKind,omitempty"`
}

type ZoneDecision struct {
	ZoneID          string   `json:"zoneID"`
	Passed          bool     `json:"passed"`
	RequiredPoints  int      `json:"requiredPoints"`
	MeasuredPoints  int      `json:"measuredPoints"`
	FailedPointIDs  []string `json:"failedPointIDs"`
	MissingPointIDs []string `json:"missingPointIDs"`
	InvalidPointIDs []string `json:"invalidPointIDs"`
	MissingCount    int      `json:"missingCount"`
	InvalidCount    int      `json:"invalidCount"`
	FailedCount     int      `json:"failedCount"`
}

type CaseDecision struct {
	RuleVersion     string          `json:"ruleVersion"`
	Passed          bool            `json:"passed"`
	CalculatedAt    time.Time       `json:"calculatedAt"`
	Points          []PointDecision `json:"points"`
	Zones           []ZoneDecision  `json:"zones"`
	FailedZoneIDs   []string        `json:"failedZoneIDs"`
	MissingPointIDs []string        `json:"missingPointIDs"`
	InvalidPointIDs []string        `json:"invalidPointIDs"`
}
