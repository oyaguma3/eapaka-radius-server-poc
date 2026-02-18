package ui

import "fmt"

// Pagination はページネーション状態を管理する。
type Pagination struct {
	TotalItems  int
	PageSize    int
	CurrentPage int
}

// 定数
const (
	// DefaultPageSize はリスト画面のデフォルトページサイズ
	DefaultPageSize = 50
	// DetailPageSize は詳細一覧画面のデフォルトページサイズ
	DetailPageSize = 10
)

// NewPagination は新しいPaginationを生成する。
func NewPagination(pageSize int) *Pagination {
	return &Pagination{
		TotalItems:  0,
		PageSize:    pageSize,
		CurrentPage: 1,
	}
}

// SetTotalItems は総アイテム数を設定する。
func (p *Pagination) SetTotalItems(total int) {
	p.TotalItems = total
	// ページ番号が範囲外になった場合は調整
	if p.CurrentPage > p.TotalPages() {
		p.CurrentPage = p.TotalPages()
	}
	if p.CurrentPage < 1 {
		p.CurrentPage = 1
	}
}

// TotalPages は総ページ数を返す。
func (p *Pagination) TotalPages() int {
	if p.TotalItems == 0 {
		return 1
	}
	pages := p.TotalItems / p.PageSize
	if p.TotalItems%p.PageSize > 0 {
		pages++
	}
	return pages
}

// StartIndex は現在のページの開始インデックスを返す。
func (p *Pagination) StartIndex() int {
	return (p.CurrentPage - 1) * p.PageSize
}

// EndIndex は現在のページの終了インデックス（排他的）を返す。
func (p *Pagination) EndIndex() int {
	end := p.CurrentPage * p.PageSize
	if end > p.TotalItems {
		end = p.TotalItems
	}
	return end
}

// HasNextPage は次のページがあるかどうかを返す。
func (p *Pagination) HasNextPage() bool {
	return p.CurrentPage < p.TotalPages()
}

// HasPrevPage は前のページがあるかどうかを返す。
func (p *Pagination) HasPrevPage() bool {
	return p.CurrentPage > 1
}

// NextPage は次のページに移動する。
func (p *Pagination) NextPage() bool {
	if p.HasNextPage() {
		p.CurrentPage++
		return true
	}
	return false
}

// PrevPage は前のページに移動する。
func (p *Pagination) PrevPage() bool {
	if p.HasPrevPage() {
		p.CurrentPage--
		return true
	}
	return false
}

// FirstPage は最初のページに移動する。
func (p *Pagination) FirstPage() {
	p.CurrentPage = 1
}

// LastPage は最後のページに移動する。
func (p *Pagination) LastPage() {
	p.CurrentPage = p.TotalPages()
}

// GoToPage は指定されたページに移動する。
func (p *Pagination) GoToPage(page int) bool {
	if page >= 1 && page <= p.TotalPages() {
		p.CurrentPage = page
		return true
	}
	return false
}

// GetPageItems はスライスから現在のページのアイテムを取得する。
func GetPageItems[T any](items []T, p *Pagination) []T {
	p.SetTotalItems(len(items))
	start := p.StartIndex()
	end := p.EndIndex()
	if start >= len(items) {
		return []T{}
	}
	return items[start:end]
}

// FormatPageInfo はページ情報の文字列を生成する。
func (p *Pagination) FormatPageInfo() string {
	if p.TotalItems == 0 {
		return "No items"
	}
	start := p.StartIndex() + 1
	end := p.EndIndex()
	return fmt.Sprintf("%d-%d of %d (Page %d/%d)", start, end, p.TotalItems, p.CurrentPage, p.TotalPages())
}

// FormatShortPageInfo は短いページ情報の文字列を生成する。
func (p *Pagination) FormatShortPageInfo() string {
	return fmt.Sprintf("%d/%d", p.CurrentPage, p.TotalPages())
}
