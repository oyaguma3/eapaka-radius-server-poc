package validation

import (
	"fmt"
	"strings"
)

// ClientValidationError はRADIUSクライアントバリデーションエラーを表す。
type ClientValidationError struct {
	Field   string
	Message string
}

func (e *ClientValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateIPv4 はIPv4アドレスのバリデーションを行う。
func ValidateIPv4(ip string) error {
	if ip == "" {
		return &ClientValidationError{Field: "IP", Message: "required"}
	}
	if !IPv4Pattern.MatchString(ip) {
		return &ClientValidationError{Field: "IP", Message: "must be a valid IPv4 address"}
	}
	return nil
}

// ValidateSecret はRADIUSシークレットのバリデーションを行う。
func ValidateSecret(secret string) error {
	if secret == "" {
		return &ClientValidationError{Field: "Secret", Message: "required"}
	}
	if len(secret) > MaxSecretLength {
		return &ClientValidationError{Field: "Secret", Message: fmt.Sprintf("must be at most %d characters", MaxSecretLength)}
	}
	if !SecretPattern.MatchString(secret) {
		return &ClientValidationError{Field: "Secret", Message: "must contain only printable ASCII characters (no spaces)"}
	}
	return nil
}

// ValidateClientName はクライアント名のバリデーションを行う。
func ValidateClientName(name string) error {
	if name == "" {
		return &ClientValidationError{Field: "Name", Message: "required"}
	}
	if len(name) > MaxClientNameLength {
		return &ClientValidationError{Field: "Name", Message: fmt.Sprintf("must be at most %d characters", MaxClientNameLength)}
	}
	if !ClientNamePattern.MatchString(name) {
		return &ClientValidationError{Field: "Name", Message: "must contain only alphanumeric characters, hyphens, and underscores"}
	}
	return nil
}

// ValidateVendor はベンダー名のバリデーションを行う。
func ValidateVendor(vendor string) error {
	if len(vendor) > MaxVendorLength {
		return &ClientValidationError{Field: "Vendor", Message: fmt.Sprintf("must be at most %d characters", MaxVendorLength)}
	}
	if vendor != "" && !VendorPattern.MatchString(vendor) {
		return &ClientValidationError{Field: "Vendor", Message: "must contain only alphanumeric characters, spaces, and hyphens"}
	}
	return nil
}

// ClientInput はRADIUSクライアントの入力データを表す。
type ClientInput struct {
	IP     string
	Secret string
	Name   string
	Vendor string
}

// ValidateClient はRADIUSクライアントデータの全体バリデーションを行う。
func ValidateClient(input *ClientInput) []error {
	var errs []error

	if err := ValidateIPv4(input.IP); err != nil {
		errs = append(errs, err)
	}
	if err := ValidateSecret(input.Secret); err != nil {
		errs = append(errs, err)
	}
	if err := ValidateClientName(input.Name); err != nil {
		errs = append(errs, err)
	}
	if err := ValidateVendor(input.Vendor); err != nil {
		errs = append(errs, err)
	}

	return errs
}

// NormalizeClientInput は入力データを正規化する。
func NormalizeClientInput(input *ClientInput) *ClientInput {
	return &ClientInput{
		IP:     strings.TrimSpace(input.IP),
		Secret: strings.TrimSpace(input.Secret),
		Name:   strings.TrimSpace(input.Name),
		Vendor: strings.TrimSpace(input.Vendor),
	}
}
