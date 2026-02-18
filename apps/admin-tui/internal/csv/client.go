package csv

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/validation"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

// ClientCSVHeader はRADIUSクライアントCSVのヘッダー行
var ClientCSVHeader = []string{"ip", "secret", "name", "vendor"}

// ParseClientCSV はRADIUSクライアントCSVをパースする。
// 全件バリデーションを行い、エラーがあれば行番号とエラーを返す。
func ParseClientCSV(r io.Reader) ([]*model.RadiusClient, []error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	// ヘッダー行を読み込み
	header, err := reader.Read()
	if err != nil {
		return nil, []error{fmt.Errorf("failed to read header: %w", err)}
	}

	// ヘッダー検証
	if err := validateClientHeader(header); err != nil {
		return nil, []error{err}
	}

	var clients []*model.RadiusClient
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

		client, parseErrs := parseClientRecord(record, lineNum)
		if len(parseErrs) > 0 {
			errs = append(errs, parseErrs...)
			continue
		}

		clients = append(clients, client)
	}

	return clients, errs
}

func validateClientHeader(header []string) error {
	if len(header) < 3 {
		return errors.New("invalid header: expected at least 3 columns (ip, secret, name)")
	}

	expected := ClientCSVHeader[:3] // ip, secret, name are required
	for i, col := range expected {
		if strings.ToLower(strings.TrimSpace(header[i])) != col {
			return fmt.Errorf("invalid header: expected '%s' at column %d, got '%s'", col, i+1, header[i])
		}
	}

	return nil
}

func parseClientRecord(record []string, lineNum int) (*model.RadiusClient, []error) {
	if len(record) < 3 {
		return nil, []error{fmt.Errorf("line %d: expected at least 3 columns, got %d", lineNum, len(record))}
	}

	vendor := ""
	if len(record) > 3 {
		vendor = record[3]
	}

	input := &validation.ClientInput{
		IP:     record[0],
		Secret: record[1],
		Name:   record[2],
		Vendor: vendor,
	}

	// 正規化
	input = validation.NormalizeClientInput(input)

	// バリデーション
	validationErrs := validation.ValidateClient(input)
	if len(validationErrs) > 0 {
		var errs []error
		for _, e := range validationErrs {
			errs = append(errs, fmt.Errorf("line %d: %s", lineNum, e.Error()))
		}
		return nil, errs
	}

	return &model.RadiusClient{
		IP:     input.IP,
		Secret: input.Secret,
		Name:   input.Name,
		Vendor: input.Vendor,
	}, nil
}

// WriteClientCSV はRADIUSクライアントデータをCSV形式で書き込む。
func WriteClientCSV(w io.Writer, clients []*model.RadiusClient) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// ヘッダー書き込み
	if err := writer.Write(ClientCSVHeader); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// データ書き込み
	for _, client := range clients {
		record := []string{client.IP, client.Secret, client.Name, client.Vendor}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write record for IP %s: %w", client.IP, err)
		}
	}

	return writer.Error()
}
