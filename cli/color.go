package cli

// 颜色常量的别名，真源在 qmicode-go/cli/term。
// 保留本文件是为了让 cli 包内既有的 Red/Cyan/... 引用无需改动。
import "qmicode-go/cli/term"

const (
	// ========== 重置 ==========
	Reset = term.Reset

	// ========== 标准前景色（较暗） ==========
	Black   = term.Black
	Red     = term.Red
	Green   = term.Green
	Yellow  = term.Yellow
	Blue    = term.Blue
	Magenta = term.Magenta
	Cyan    = term.Cyan
	White   = term.White

	// ========== 亮前景色（高亮） ==========
	BrightBlack   = term.BrightBlack
	BrightRed     = term.BrightRed
	BrightGreen   = term.BrightGreen
	BrightYellow  = term.BrightYellow
	BrightBlue    = term.BrightBlue
	BrightMagenta = term.BrightMagenta
	BrightCyan    = term.BrightCyan
	BrightWhite   = term.BrightWhite
	LightGray     = term.LightGray

	// ========== 标准背景色 ==========
	BgBlack   = term.BgBlack
	BgRed     = term.BgRed
	BgGreen   = term.BgGreen
	BgYellow  = term.BgYellow
	BgBlue    = term.BgBlue
	BgMagenta = term.BgMagenta
	BgCyan    = term.BgCyan
	BgWhite   = term.BgWhite

	// ========== 亮背景色 ==========
	BgBrightBlack   = term.BgBrightBlack
	BgBrightRed     = term.BgBrightRed
	BgBrightGreen   = term.BgBrightGreen
	BgBrightYellow  = term.BgBrightYellow
	BgBrightBlue    = term.BgBrightBlue
	BgBrightMagenta = term.BgBrightMagenta
	BgBrightCyan    = term.BgBrightCyan
	BgBrightWhite   = term.BgBrightWhite

	// ========== 文本样式 ==========
	Bold          = term.Bold
	Dim           = term.Dim
	Italic        = term.Italic
	Underline     = term.Underline
	Blink         = term.Blink
	FastBlink     = term.FastBlink
	Reverse       = term.Reverse
	Hidden        = term.Hidden
	Strikethrough = term.Strikethrough

	// ========== 样式关闭（单独取消） ==========
	ResetBold          = term.ResetBold
	ResetItalic        = term.ResetItalic
	ResetUnderline     = term.ResetUnderline
	ResetBlink         = term.ResetBlink
	ResetReverse       = term.ResetReverse
	ResetHidden        = term.ResetHidden
	ResetStrikethrough = term.ResetStrikethrough
)
