package validation

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/model"
)

// PolicyValidationError はポリシーバリデーションエラーを表す。
type PolicyValidationError struct {
	Field   string
	Message string
}

func (e *PolicyValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateDefaultAction はデフォルトアクションのバリデーションを行う。
func ValidateDefaultAction(action string) error {
	if action == "" {
		return &PolicyValidationError{Field: "Default", Message: "required"}
	}
	if action != "allow" && action != "deny" {
		return &PolicyValidationError{Field: "Default", Message: "must be 'allow' or 'deny'"}
	}
	return nil
}

// ValidateNasID はNAS IDのバリデーションを行う。
func ValidateNasID(nasID string) error {
	if nasID == "" {
		return &PolicyValidationError{Field: "NasID", Message: "required"}
	}
	if len(nasID) > MaxNasIDLength {
		return &PolicyValidationError{Field: "NasID", Message: fmt.Sprintf("must be at most %d characters", MaxNasIDLength)}
	}
	if !NasIDPattern.MatchString(nasID) {
		return &PolicyValidationError{Field: "NasID", Message: "must contain only printable ASCII characters and wildcards"}
	}
	return nil
}

// ValidateSSID はSSIDのバリデーションを行う。
func ValidateSSID(ssid string) error {
	if ssid == "" {
		return &PolicyValidationError{Field: "SSID", Message: "required"}
	}
	if len(ssid) > MaxSSIDLength {
		return &PolicyValidationError{Field: "SSID", Message: fmt.Sprintf("must be at most %d characters", MaxSSIDLength)}
	}
	return nil
}

// ValidateAllowedSSIDs は許可SSIDリストのバリデーションを行う。
func ValidateAllowedSSIDs(ssids []string) error {
	if len(ssids) == 0 {
		return &PolicyValidationError{Field: "AllowedSSIDs", Message: "at least one SSID required"}
	}
	for i, ssid := range ssids {
		if err := ValidateSSID(ssid); err != nil {
			return &PolicyValidationError{
				Field:   fmt.Sprintf("AllowedSSIDs[%d]", i),
				Message: err.Error(),
			}
		}
	}
	return nil
}

// ValidateVlanID はVLAN IDのバリデーションを行う。
func ValidateVlanID(vlanID string) error {
	if vlanID == "" {
		return nil // 未設定OK
	}
	v, err := strconv.Atoi(vlanID)
	if err != nil {
		return &PolicyValidationError{Field: "VlanID", Message: "must be a valid number"}
	}
	if v < 0 {
		return &PolicyValidationError{Field: "VlanID", Message: "must be non-negative"}
	}
	if v > MaxVlanID {
		return &PolicyValidationError{Field: "VlanID", Message: fmt.Sprintf("must be at most %d", MaxVlanID)}
	}
	return nil
}

// ValidateSessionTimeout はセッションタイムアウトのバリデーションを行う。
func ValidateSessionTimeout(timeout int) error {
	if timeout < 0 {
		return &PolicyValidationError{Field: "SessionTimeout", Message: "must be non-negative"}
	}
	if timeout > MaxSessionTimeout {
		return &PolicyValidationError{Field: "SessionTimeout", Message: fmt.Sprintf("must be at most %d seconds", MaxSessionTimeout)}
	}
	return nil
}

// ValidatePolicyRule はポリシールールのバリデーションを行う。
func ValidatePolicyRule(rule *model.PolicyRule) []error {
	var errs []error

	if err := ValidateNasID(rule.NasID); err != nil {
		errs = append(errs, err)
	}
	if err := ValidateAllowedSSIDs(rule.AllowedSSIDs); err != nil {
		errs = append(errs, err)
	}
	if err := ValidateVlanID(rule.VlanID); err != nil {
		errs = append(errs, err)
	}
	if err := ValidateSessionTimeout(rule.SessionTimeout); err != nil {
		errs = append(errs, err)
	}

	return errs
}

// PolicyInput はポリシーの入力データを表す。
type PolicyInput struct {
	IMSI    string
	Default string
	Rules   []model.PolicyRule
}

// ValidatePolicy はポリシーデータの全体バリデーションを行う。
func ValidatePolicy(input *PolicyInput) []error {
	var errs []error

	if err := ValidateIMSI(input.IMSI); err != nil {
		errs = append(errs, &PolicyValidationError{Field: "IMSI", Message: err.Error()})
	}
	if err := ValidateDefaultAction(input.Default); err != nil {
		errs = append(errs, err)
	}

	for i, rule := range input.Rules {
		ruleErrs := ValidatePolicyRule(&rule)
		for _, e := range ruleErrs {
			if pe, ok := e.(*PolicyValidationError); ok {
				errs = append(errs, &PolicyValidationError{
					Field:   fmt.Sprintf("Rules[%d].%s", i, pe.Field),
					Message: pe.Message,
				})
			} else {
				errs = append(errs, e)
			}
		}
	}

	return errs
}

// NormalizePolicyInput は入力データを正規化する。
func NormalizePolicyInput(input *PolicyInput) *PolicyInput {
	normalized := &PolicyInput{
		IMSI:    strings.TrimSpace(input.IMSI),
		Default: strings.ToLower(strings.TrimSpace(input.Default)),
		Rules:   make([]model.PolicyRule, len(input.Rules)),
	}

	for i, rule := range input.Rules {
		normalizedSSIDs := make([]string, len(rule.AllowedSSIDs))
		for j, ssid := range rule.AllowedSSIDs {
			normalizedSSIDs[j] = strings.TrimSpace(ssid)
		}

		normalized.Rules[i] = model.PolicyRule{
			NasID:          strings.TrimSpace(rule.NasID),
			AllowedSSIDs:   normalizedSSIDs,
			VlanID:         rule.VlanID,
			SessionTimeout: rule.SessionTimeout,
		}
	}

	return normalized
}
