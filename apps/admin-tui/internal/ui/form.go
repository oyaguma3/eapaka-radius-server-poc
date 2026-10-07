package ui

// フォームの高さの計算に使う、tview.NewForm の既定の値。
const (
	formBorderRows  = 2 // 上下の枠
	formPaddingRows = 2 // 枠の内側の上下の余白（SetBorderPadding(1, 1, 1, 1)）
	formItemPadding = 1 // 入力欄の間の空行（SetItemPadding の既定値）
	formButtonRows  = 1 // ボタンの行
)

// FormHeight は、縦並びのフォーム（tview.NewForm の既定の枠・余白・項目間隔）を、
// itemCount 個の入力欄とボタンが欠けずに表示できる高さを返す。
// 各入力欄は「欄＋空行」の2行を使い、ボタンはその後に置かれる（tview の Form.Draw）。
// 高さが足りないとボタンの行から表示されなくなる。
func FormHeight(itemCount int) int {
	return formBorderRows + formPaddingRows + itemCount*(1+formItemPadding) + formButtonRows
}
