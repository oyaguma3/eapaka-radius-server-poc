// Package csv はCSVインポート/エクスポート機能を提供する。
package csv

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/validation"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

// SubscriberCSVHeader は加入者CSVのヘッダー行
var SubscriberCSVHeader = []string{"imsi", "ki", "opc", "amf", "sqn"}

// ParseSubscriberCSV は加入者CSVをパースする。
// 全件バリデーションを行い、エラーがあれば行番号とエラーを返す。
func ParseSubscriberCSV(r io.Reader) ([]*model.Subscriber, []error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	// ヘッダー行を読み込み
	header, err := reader.Read()
	if err != nil {
		return nil, []error{fmt.Errorf("failed to read header: %w", err)}
	}

	// ヘッダー検証
	if err := validateSubscriberHeader(header); err != nil {
		return nil, []error{err}
	}

	var subscribers []*model.Subscriber
	var errs []error
	lineNum := 1 // ヘッダーを1行目としてカウント

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

		sub, parseErrs := parseSubscriberRecord(record, lineNum)
		if len(parseErrs) > 0 {
			errs = append(errs, parseErrs...)
			continue
		}

		subscribers = append(subscribers, sub)
	}

	return subscribers, errs
}

func validateSubscriberHeader(header []string) error {
	if len(header) < 5 {
		return errors.New("invalid header: expected at least 5 columns (imsi, ki, opc, amf, sqn)")
	}

	expected := SubscriberCSVHeader
	for i, col := range expected {
		if strings.ToLower(strings.TrimSpace(header[i])) != col {
			return fmt.Errorf("invalid header: expected '%s' at column %d, got '%s'", col, i+1, header[i])
		}
	}

	return nil
}

func parseSubscriberRecord(record []string, lineNum int) (*model.Subscriber, []error) {
	if len(record) < 5 {
		return nil, []error{fmt.Errorf("line %d: expected at least 5 columns, got %d", lineNum, len(record))}
	}

	input := &validation.SubscriberInput{
		IMSI: record[0],
		Ki:   record[1],
		OPc:  record[2],
		AMF:  record[3],
		SQN:  record[4],
	}

	// 正規化
	input = validation.NormalizeSubscriberInput(input)

	// バリデーション
	validationErrs := validation.ValidateSubscriber(input)
	if len(validationErrs) > 0 {
		var errs []error
		for _, e := range validationErrs {
			errs = append(errs, fmt.Errorf("line %d: %s", lineNum, e.Error()))
		}
		return nil, errs
	}

	return &model.Subscriber{
		IMSI:      input.IMSI,
		Ki:        input.Ki,
		OPc:       input.OPc,
		AMF:       input.AMF,
		SQN:       input.SQN,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// WriteSubscriberCSV は加入者データをCSV形式で書き込む。
func WriteSubscriberCSV(w io.Writer, subscribers []*model.Subscriber) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// ヘッダー書き込み
	if err := writer.Write(SubscriberCSVHeader); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// データ書き込み
	for _, sub := range subscribers {
		record := []string{sub.IMSI, sub.Ki, sub.OPc, sub.AMF, sub.SQN}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write record for IMSI %s: %w", sub.IMSI, err)
		}
	}

	return writer.Error()
}
