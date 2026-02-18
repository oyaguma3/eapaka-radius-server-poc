package validation

import (
	"fmt"
	"strings"
)

// SubscriberValidationError は加入者バリデーションエラーを表す。
type SubscriberValidationError struct {
	Field   string
	Message string
}

func (e *SubscriberValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateIMSI はIMSIのバリデーションを行う。
func ValidateIMSI(imsi string) error {
	if imsi == "" {
		return &SubscriberValidationError{Field: "IMSI", Message: "required"}
	}
	if !IMSIPattern.MatchString(imsi) {
		return &SubscriberValidationError{Field: "IMSI", Message: "must be 15 digits"}
	}
	return nil
}

// ValidateKi はKiのバリデーションを行う。
func ValidateKi(ki string) error {
	if ki == "" {
		return &SubscriberValidationError{Field: "Ki", Message: "required"}
	}
	ki = strings.ToUpper(ki)
	if !KiPattern.MatchString(ki) {
		return &SubscriberValidationError{Field: "Ki", Message: "must be 32 hex characters"}
	}
	return nil
}

// ValidateOPc はOPcのバリデーションを行う。
func ValidateOPc(opc string) error {
	if opc == "" {
		return &SubscriberValidationError{Field: "OPc", Message: "required"}
	}
	opc = strings.ToUpper(opc)
	if !OPcPattern.MatchString(opc) {
		return &SubscriberValidationError{Field: "OPc", Message: "must be 32 hex characters"}
	}
	return nil
}

// ValidateAMF はAMFのバリデーションを行う。
func ValidateAMF(amf string) error {
	if amf == "" {
		return &SubscriberValidationError{Field: "AMF", Message: "required"}
	}
	amf = strings.ToUpper(amf)
	if !AMFPattern.MatchString(amf) {
		return &SubscriberValidationError{Field: "AMF", Message: "must be 4 hex characters"}
	}
	return nil
}

// ValidateSQN はSQNのバリデーションを行う。
func ValidateSQN(sqn string) error {
	if sqn == "" {
		return &SubscriberValidationError{Field: "SQN", Message: "required"}
	}
	sqn = strings.ToUpper(sqn)
	if !SQNPattern.MatchString(sqn) {
		return &SubscriberValidationError{Field: "SQN", Message: "must be 12 hex characters"}
	}
	return nil
}

// SubscriberInput は加入者の入力データを表す。
type SubscriberInput struct {
	IMSI string
	Ki   string
	OPc  string
	AMF  string
	SQN  string
}

// ValidateSubscriber は加入者データの全体バリデーションを行う。
func ValidateSubscriber(input *SubscriberInput) []error {
	var errs []error

	if err := ValidateIMSI(input.IMSI); err != nil {
		errs = append(errs, err)
	}
	if err := ValidateKi(input.Ki); err != nil {
		errs = append(errs, err)
	}
	if err := ValidateOPc(input.OPc); err != nil {
		errs = append(errs, err)
	}
	if err := ValidateAMF(input.AMF); err != nil {
		errs = append(errs, err)
	}
	if err := ValidateSQN(input.SQN); err != nil {
		errs = append(errs, err)
	}

	return errs
}

// NormalizeSubscriberInput は入力データを正規化する（大文字化など）。
func NormalizeSubscriberInput(input *SubscriberInput) *SubscriberInput {
	return &SubscriberInput{
		IMSI: strings.TrimSpace(input.IMSI),
		Ki:   strings.ToUpper(strings.TrimSpace(input.Ki)),
		OPc:  strings.ToUpper(strings.TrimSpace(input.OPc)),
		AMF:  strings.ToUpper(strings.TrimSpace(input.AMF)),
		SQN:  strings.ToUpper(strings.TrimSpace(input.SQN)),
	}
}
