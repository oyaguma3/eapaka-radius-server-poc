package ui

import "github.com/gdamore/tcell/v2"

// 色定義
var (
	// Primary colors
	ColorPrimary       = tcell.ColorBlue
	ColorPrimaryDark   = tcell.ColorDarkBlue
	ColorPrimaryBright = tcell.ColorLightBlue

	// Status colors
	ColorSuccess = tcell.ColorGreen
	ColorWarning = tcell.ColorYellow
	ColorError   = tcell.ColorRed
	ColorInfo    = tcell.ColorTeal

	// Background colors
	ColorBackground      = tcell.ColorDefault
	ColorBackgroundAlt   = tcell.ColorDarkGray
	ColorBackgroundModal = tcell.ColorBlack

	// Text colors
	ColorText        = tcell.ColorWhite
	ColorTextMuted   = tcell.ColorGray
	ColorTextInverse = tcell.ColorBlack

	// Border colors
	ColorBorder       = tcell.ColorWhite
	ColorBorderFocus  = tcell.ColorBlue
	ColorBorderActive = tcell.ColorGreen

	// Highlight colors
	ColorHighlight       = tcell.ColorYellow
	ColorHighlightBg     = tcell.ColorDarkBlue
	ColorSelected        = tcell.ColorLightBlue
	ColorSelectedBg      = tcell.ColorNavy
	ColorPolicyMissing   = tcell.ColorYellow
	ColorPolicyMissingBg = tcell.ColorDefault
)

// スタイル定義
const (
	// Border characters
	BorderVertical    = '│'
	BorderHorizontal  = '─'
	BorderTopLeft     = '┌'
	BorderTopRight    = '┐'
	BorderBottomLeft  = '└'
	BorderBottomRight = '┘'

	// Indicator characters
	IndicatorPolicyMissing = '!'
	IndicatorSelected      = '>'
	IndicatorExpanded      = '▼'
	IndicatorCollapsed     = '▶'
)

// StyleText はテキストのスタイルを適用した文字列を返す。
func StyleText(text string, color tcell.Color) string {
	return "[" + colorToHex(color) + "]" + text + "[-]"
}

// StyleBold は太字スタイルを適用した文字列を返す。
func StyleBold(text string) string {
	return "[::b]" + text + "[::-]"
}

// StyleUnderline は下線スタイルを適用した文字列を返す。
func StyleUnderline(text string) string {
	return "[::u]" + text + "[::-]"
}

// StyleDim は薄い色のスタイルを適用した文字列を返す。
func StyleDim(text string) string {
	return "[::d]" + text + "[::-]"
}

// StyleHighlight はハイライトスタイルを適用した文字列を返す。
func StyleHighlight(text string) string {
	return "[yellow::b]" + text + "[-::-]"
}

// StyleSuccess は成功スタイルを適用した文字列を返す。
func StyleSuccess(text string) string {
	return "[green]" + text + "[-]"
}

// StyleError はエラースタイルを適用した文字列を返す。
func StyleError(text string) string {
	return "[red]" + text + "[-]"
}

// StyleWarning は警告スタイルを適用した文字列を返す。
func StyleWarning(text string) string {
	return "[yellow]" + text + "[-]"
}

// StyleInfo は情報スタイルを適用した文字列を返す。
func StyleInfo(text string) string {
	return "[teal]" + text + "[-]"
}

// colorToHex はtcell.Colorを16進数文字列に変換する。
func colorToHex(color tcell.Color) string {
	switch color {
	case tcell.ColorWhite:
		return "white"
	case tcell.ColorBlack:
		return "black"
	case tcell.ColorRed:
		return "red"
	case tcell.ColorGreen:
		return "green"
	case tcell.ColorBlue:
		return "blue"
	case tcell.ColorYellow:
		return "yellow"
	case tcell.ColorTeal:
		return "teal"
	case tcell.ColorGray:
		return "gray"
	case tcell.ColorDarkGray:
		return "darkgray"
	default:
		return "white"
	}
}
