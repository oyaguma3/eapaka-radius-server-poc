package csv

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/model"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/validation"
)

// PolicyCSVHeader はポリシーCSVのヘッダー行
var PolicyCSVHeader = []string{"imsi", "default", "rules_json"}

// ParsePolicyCSV はポリシーCSVをパースする。
// 全件バリデーションを行い、エラーがあれば行番号とエラーを返す。
func ParsePolicyCSV(r io.Reader) ([]*model.Policy, []error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	// ヘッダー行を読み込み
	header, err := reader.Read()
	if err != nil {
		return nil, []error{fmt.Errorf("failed to read header: %w", err)}
	}

	// ヘッダー検証
	if err := validatePolicyHeader(header); err != nil {
		return nil, []error{err}
	}

	var policies []*model.Policy
	var errs []error
	lineNum := 1

	for {
		lineNum++
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("line %d: %w", lineNum, err))
			continue
		}

		policy, parseErrs := parsePolicyRecord(record, lineNum)
		if len(parseErrs) > 0 {
			errs = append(errs, parseErrs...)
			continue
		}

		policies = append(policies, policy)
	}

	return policies, errs
}

func validatePolicyHeader(header []string) error {
	if len(header) < 3 {
		return errors.New("invalid header: expected at least 3 columns (imsi, default, rules_json)")
	}

	expected := PolicyCSVHeader
	for i, col := range expected {
		if strings.ToLower(strings.TrimSpace(header[i])) != col {
			return fmt.Errorf("invalid header: expected '%s' at column %d, got '%s'", col, i+1, header[i])
		}
	}

	return nil
}

func parsePolicyRecord(record []string, lineNum int) (*model.Policy, []error) {
	if len(record) < 3 {
		return nil, []error{fmt.Errorf("line %d: expected at least 3 columns, got %d", lineNum, len(record))}
	}

	imsi := strings.TrimSpace(record[0])
	defaultAction := strings.ToLower(strings.TrimSpace(record[1]))
	rulesJSON := strings.TrimSpace(record[2])

	// IMSI検証
	if err := validation.ValidateIMSI(imsi); err != nil {
		return nil, []error{fmt.Errorf("line %d: %s", lineNum, err.Error())}
	}

	// Default検証
	if err := validation.ValidateDefaultAction(defaultAction); err != nil {
		return nil, []error{fmt.Errorf("line %d: %s", lineNum, err.Error())}
	}

	// Rules JSONパース
	var rules []model.PolicyRule
	if rulesJSON != "" && rulesJSON != "[]" {
		if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
			return nil, []error{fmt.Errorf("line %d: invalid rules_json: %w", lineNum, err)}
		}

		// 各ルールを検証
		for i, rule := range rules {
			if ruleErrs := validation.ValidatePolicyRule(&rule); len(ruleErrs) > 0 {
				var errs []error
				for _, e := range ruleErrs {
					errs = append(errs, fmt.Errorf("line %d, rule[%d]: %s", lineNum, i, e.Error()))
				}
				return nil, errs
			}
		}
	}

	return &model.Policy{
		IMSI:      imsi,
		Default:   defaultAction,
		RulesJSON: rulesJSON,
		Rules:     rules,
	}, nil
}

// WritePolicyCSV はポリシーデータをCSV形式で書き込む。
func WritePolicyCSV(w io.Writer, policies []*model.Policy) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// ヘッダー書き込み
	if err := writer.Write(PolicyCSVHeader); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// データ書き込み
	for _, policy := range policies {
		// RulesをJSONにエンコード
		rulesJSON := "[]"
		if len(policy.Rules) > 0 {
			data, err := json.Marshal(policy.Rules)
			if err != nil {
				return fmt.Errorf("failed to encode rules for IMSI %s: %w", policy.IMSI, err)
			}
			rulesJSON = string(data)
		} else if policy.RulesJSON != "" {
			rulesJSON = policy.RulesJSON
		}

		record := []string{policy.IMSI, policy.Default, rulesJSON}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write record for IMSI %s: %w", policy.IMSI, err)
		}
	}

	return writer.Error()
}
