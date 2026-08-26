package measurement

import (
	"strings"
	"time"
	"voice-clarity-acceptance/internal/domain"
)

const RuleVersion = "VC-ACCEPT-2026.1"

type ReadingInput struct {
	BackgroundNoiseDBA   float64
	BroadcastLevelDBA    float64
	IntelligibilityValue float64
	InstrumentID         string
	MeasuredAt           time.Time
}

func ValidateReading(input ReadingInput, now time.Time) (domain.ValidityStatus, string) {
	if err := ValidateReadingFields(input, now); err != nil {
		return domain.ReadingInvalid, err.Error()
	}
	if input.BroadcastLevelDBA-input.BackgroundNoiseDBA < 3 {
		return domain.ReadingInvalid, "广播声压与背景噪声差值不足 3 dB"
	}
	return domain.ReadingValid, ""
}

func ValidateReadingFields(input ReadingInput, now time.Time) error {
	if input.BackgroundNoiseDBA < 15 || input.BackgroundNoiseDBA > 120 {
		return &domain.FieldError{Field: "backgroundNoiseDBA", Message: "必须在 15 至 120 dBA 之间"}
	}
	if input.BroadcastLevelDBA < 30 || input.BroadcastLevelDBA > 130 {
		return &domain.FieldError{Field: "broadcastLevelDBA", Message: "必须在 30 至 130 dBA 之间"}
	}
	if input.IntelligibilityValue < 0 || input.IntelligibilityValue > 1 {
		return &domain.FieldError{Field: "intelligibilityValue", Message: "必须在 0 至 1 之间"}
	}
	if strings.TrimSpace(input.InstrumentID) == "" {
		return &domain.FieldError{Field: "instrumentID", Message: "不能为空"}
	}
	if input.MeasuredAt.IsZero() {
		return &domain.FieldError{Field: "measuredAt", Message: "不能为空"}
	}
	if input.MeasuredAt.After(now.Add(5 * time.Minute)) {
		return &domain.FieldError{Field: "measuredAt", Message: "不能晚于当前时间五分钟以上"}
	}
	if input.MeasuredAt.Before(now.AddDate(-1, 0, 0)) {
		return &domain.FieldError{Field: "measuredAt", Message: "不能早于一年以前"}
	}
	return nil
}

func BuildReading(id, roundID, pointID string, input ReadingInput, now time.Time) domain.MeasurementReading {
	status, reason := ValidateReading(input, now)
	return domain.MeasurementReading{ID: id, RoundID: roundID, PointID: pointID, BackgroundNoiseDBA: input.BackgroundNoiseDBA, BroadcastLevelDBA: input.BroadcastLevelDBA, IntelligibilityValue: input.IntelligibilityValue, InstrumentID: strings.TrimSpace(input.InstrumentID), MeasuredAt: input.MeasuredAt, ValidityStatus: status, InvalidReason: reason}
}
