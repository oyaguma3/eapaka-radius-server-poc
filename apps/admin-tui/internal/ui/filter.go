package ui

import (
	"strings"
)

// Filter はクライアント側フィルタを管理する。
type Filter struct {
	Query   string
	Active  bool
	Columns []string // フィルタ対象のカラム名
}

// NewFilter は新しいFilterを生成する。
func NewFilter(columns ...string) *Filter {
	return &Filter{
		Query:   "",
		Active:  false,
		Columns: columns,
	}
}

// SetQuery はフィルタクエリを設定する。
func (f *Filter) SetQuery(query string) {
	f.Query = strings.TrimSpace(query)
	f.Active = f.Query != ""
}

// Clear はフィルタをクリアする。
func (f *Filter) Clear() {
	f.Query = ""
	f.Active = false
}

// Match は値がフィルタクエリにマッチするかどうかを返す。
// クエリは大文字小文字を区別しない部分一致。
func (f *Filter) Match(value string) bool {
	if !f.Active {
		return true
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(f.Query))
}

// MatchAny は複数の値のいずれかがフィルタクエリにマッチするかどうかを返す。
func (f *Filter) MatchAny(values ...string) bool {
	if !f.Active {
		return true
	}
	query := strings.ToLower(f.Query)
	for _, v := range values {
		if strings.Contains(strings.ToLower(v), query) {
			return true
		}
	}
	return false
}

// FilterItems はスライスからフィルタ条件にマッチするアイテムを抽出する。
func FilterItems[T any](items []T, filter *Filter, getValues func(T) []string) []T {
	if !filter.Active {
		return items
	}

	var result []T
	for _, item := range items {
		values := getValues(item)
		if filter.MatchAny(values...) {
			result = append(result, item)
		}
	}
	return result
}

// FormatFilterStatus はフィルタの状態を文字列で返す。
func (f *Filter) FormatFilterStatus() string {
	if !f.Active {
		return ""
	}
	return "Filter: \"" + f.Query + "\""
}
