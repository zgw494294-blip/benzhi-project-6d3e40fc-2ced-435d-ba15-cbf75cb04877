package domain

import "time"

type AcceptanceCase struct {
	ID                   string                 `json:"id"`
	CaseNumber           string                 `json:"caseNumber"`
	SiteName             string                 `json:"siteName"`
	Status               CaseStatus             `json:"status"`
	Version              int64                  `json:"version"`
	ResponsibleEngineer  string                 `json:"responsibleEngineer"`
	CreatedAt            time.Time              `json:"createdAt"`
	UpdatedAt            time.Time              `json:"updatedAt"`
	PlanRevision         int                    `json:"planRevision"`
	PlanDigest           string                 `json:"planDigest,omitempty"`
	PlanCandidateSummary string                 `json:"planCandidateSummary,omitempty"`
	PlanPrecheckRevision int                    `json:"planPrecheckRevision,omitempty"`
	PlanRevisions        []PlanRevisionSnapshot `json:"planRevisions,omitempty"`
	RuleVersion          string                 `json:"ruleVersion"`
	Zones                []BroadcastZone        `json:"zones"`
	Points               []MeasurementPoint     `json:"points"`
	Rounds               []MeasurementRound     `json:"rounds"`
	Deviations           []Deviation            `json:"deviations"`
	Reviews              []ReviewRecord         `json:"reviews"`
	Decision             *CaseDecision          `json:"decision,omitempty"`
	Credential           *AcceptanceCredential  `json:"credential,omitempty"`
}

type PlanRevisionSnapshot struct {
	Revision    int                `json:"revision"`
	Points      []MeasurementPoint `json:"points"`
	SubmittedBy string             `json:"submittedBy"`
	SubmittedAt time.Time          `json:"submittedAt"`
	Digest      string             `json:"digest"`
}

type BroadcastZone struct {
	ID                       string  `json:"id"`
	CaseID                   string  `json:"caseID"`
	Name                     string  `json:"name"`
	UsageClass               string  `json:"usageClass"`
	AreaSquareMeters         float64 `json:"areaSquareMeters"`
	MinimumPointCount        int     `json:"minimumPointCount"`
	IntelligibilityThreshold float64 `json:"intelligibilityThreshold"`
	Notes                    string  `json:"notes,omitempty"`
}

type MeasurementPoint struct {
	ID                  string `json:"id"`
	ZoneID              string `json:"zoneID"`
	PointCode           string `json:"pointCode"`
	LocationDescription string `json:"locationDescription"`
	CoverageTag         string `json:"coverageTag"`
	Sequence            int    `json:"sequence"`
	PlanRevision        int    `json:"planRevision"`
}

type MeasurementRound struct {
	ID             string               `json:"id"`
	CaseID         string               `json:"caseID"`
	Number         int                  `json:"number"`
	Kind           string               `json:"kind"`
	Status         RoundStatus          `json:"status"`
	TargetPointIDs []string             `json:"targetPointIDs"`
	Readings       []MeasurementReading `json:"readings"`
	CreatedAt      time.Time            `json:"createdAt"`
	ClosedAt       *time.Time           `json:"closedAt,omitempty"`
}

type MeasurementReading struct {
	ID                    string         `json:"id"`
	RoundID               string         `json:"roundID"`
	PointID               string         `json:"pointID"`
	BackgroundNoiseDBA    float64        `json:"backgroundNoiseDBA"`
	BroadcastLevelDBA     float64        `json:"broadcastLevelDBA"`
	IntelligibilityValue  float64        `json:"intelligibilityValue"`
	InstrumentID          string         `json:"instrumentID"`
	MeasuredAt            time.Time      `json:"measuredAt"`
	ValidityStatus        ValidityStatus `json:"validityStatus"`
	InvalidReason         string         `json:"invalidReason,omitempty"`
	Revision              int            `json:"revision,omitempty"`
	SupersedesReadingID   string         `json:"supersedesReadingID,omitempty"`
	SupersededByReadingID string         `json:"supersededByReadingID,omitempty"`
	CorrectionReason      string         `json:"correctionReason,omitempty"`
	Operator              string         `json:"operator,omitempty"`
}

type Deviation struct {
	ID                    string            `json:"id"`
	CaseID                string            `json:"caseID"`
	ZoneID                string            `json:"zoneID"`
	Reason                string            `json:"reason"`
	CorrectiveAction      string            `json:"correctiveAction"`
	TargetPointIDs        []string          `json:"targetPointIDs"`
	Status                DeviationStatus   `json:"status"`
	OpenedAt              time.Time         `json:"openedAt"`
	ClosedAt              *time.Time        `json:"closedAt,omitempty"`
	SourceDecisionVersion int64             `json:"sourceDecisionVersion,omitempty"`
	SourceRuleVersion     string            `json:"sourceRuleVersion,omitempty"`
	ExecutionRecords      []ExecutionRecord `json:"executionRecords,omitempty"`
}

type ReviewRecord struct {
	ID          string        `json:"id"`
	Decision    string        `json:"decision"`
	Reviewer    string        `json:"reviewer"`
	Comment     string        `json:"comment"`
	CaseVersion int64         `json:"caseVersion"`
	ReviewedAt  time.Time     `json:"reviewedAt"`
	Issues      []ReviewIssue `json:"issues,omitempty"`
}

type ReviewIssue struct {
	ID          string           `json:"id"`
	Category    string           `json:"category"`
	Description string           `json:"description"`
	Requirement string           `json:"requirement"`
	ZoneID      string           `json:"zoneID,omitempty"`
	PointID     string           `json:"pointID,omitempty"`
	RoundID     string           `json:"roundID,omitempty"`
	Resolved    bool             `json:"resolved"`
	Resolution  *IssueResolution `json:"resolution,omitempty"`
}

type IssueResolution struct {
	Resolver    string    `json:"resolver"`
	ResolvedAt  time.Time `json:"resolvedAt"`
	Explanation string    `json:"explanation"`
	DeviationID string    `json:"deviationID,omitempty"`
	RoundID     string    `json:"roundID,omitempty"`
	ReadingID   string    `json:"readingID,omitempty"`
}

type ExecutionRecord struct {
	ID          string    `json:"id"`
	DeviationID string    `json:"deviationID"`
	Content     string    `json:"content"`
	Operator    string    `json:"operator"`
	CompletedAt time.Time `json:"completedAt"`
	PointIDs    []string  `json:"pointIDs"`
}

type AcceptanceCredential struct {
	ID                  string    `json:"id"`
	CaseID              string    `json:"caseID"`
	CaseVersion         int64     `json:"caseVersion"`
	Decision            string    `json:"decision"`
	Reviewer            string    `json:"reviewer"`
	IssuedAt            time.Time `json:"issuedAt"`
	Sequence            uint64    `json:"sequence"`
	SnapshotDigest      string    `json:"snapshotDigest"`
	PreviousAuditDigest string    `json:"previousAuditDigest"`
	IssuedAuditSequence uint64    `json:"issuedAuditSequence,omitempty"`
}
