package term

// ANSI 颜色常量定义
const (
	// ========== 重置 ==========
	Reset = "\x1B[0m" // 重置所有样式

	// ========== 标准前景色（较暗） ==========
	Black   = "\x1B[30m" // 黑色
	Red     = "\x1B[31m" // 红色
	Green   = "\x1B[32m" // 绿色
	Yellow  = "\x1B[33m" // 黄色
	Blue    = "\x1B[34m" // 蓝色
	Magenta = "\x1B[35m" // 品红色（洋红）
	Cyan    = "\x1B[36m" // 青色（蓝绿色）
	White   = "\x1B[37m" // 白色（实际多为灰色）

	// ========== 亮前景色（高亮） ==========
	BrightBlack   = "\x1B[90m" // 亮黑色（深灰色）
	BrightRed     = "\x1B[91m" // 亮红色
	BrightGreen   = "\x1B[92m" // 亮绿色
	BrightYellow  = "\x1B[93m" // 亮黄色
	BrightBlue    = "\x1B[94m" // 亮蓝色
	BrightMagenta = "\x1B[95m" // 亮品红色
	BrightCyan    = "\x1B[96m" // 亮青色
	BrightWhite   = "\x1B[97m" // 亮白色（真白）
	LightGray     = "\x1B[97m" // 亮灰色（同 BrightWhite）

	// ========== 标准背景色 ==========
	BgBlack   = "\x1B[40m" // 黑色背景
	BgRed     = "\x1B[41m" // 红色背景
	BgGreen   = "\x1B[42m" // 绿色背景
	BgYellow  = "\x1B[43m" // 黄色背景
	BgBlue    = "\x1B[44m" // 蓝色背景
	BgMagenta = "\x1B[45m" // 品红色背景
	BgCyan    = "\x1B[46m" // 青色背景
	BgWhite   = "\x1B[47m" // 白色背景

	// ========== 亮背景色 ==========
	BgBrightBlack   = "\x1B[100m" // 亮黑色背景（深灰背景）
	BgBrightRed     = "\x1B[101m" // 亮红色背景
	BgBrightGreen   = "\x1B[102m" // 亮绿色背景
	BgBrightYellow  = "\x1B[103m" // 亮黄色背景
	BgBrightBlue    = "\x1B[104m" // 亮蓝色背景
	BgBrightMagenta = "\x1B[105m" // 亮品红色背景
	BgBrightCyan    = "\x1B[106m" // 亮青色背景
	BgBrightWhite   = "\x1B[107m" // 亮白色背景

	// ========== 文本样式 ==========
	Bold          = "\x1B[1m" // 加粗（高亮）
	Dim           = "\x1B[2m" // 暗淡（细体/低亮度）
	Italic        = "\x1B[3m" // 斜体（部分终端不支持）
	Underline     = "\x1B[4m" // 下划线
	Blink         = "\x1B[5m" // 缓慢闪烁（部分终端不支持）
	FastBlink     = "\x1B[6m" // 快速闪烁（很少支持）
	Reverse       = "\x1B[7m" // 反色显示（前景背景互换）
	Hidden        = "\x1B[8m" // 隐藏文字（不可见）
	Strikethrough = "\x1B[9m" // 删除线（部分终端不支持）

	// ========== 样式关闭（单独取消） ==========
	ResetBold          = "\x1B[22m" // 关闭加粗/暗淡
	ResetItalic        = "\x1B[23m" // 关闭斜体
	ResetUnderline     = "\x1B[24m" // 关闭下划线
	ResetBlink         = "\x1B[25m" // 关闭闪烁
	ResetReverse       = "\x1B[27m" // 关闭反色
	ResetHidden        = "\x1B[28m" // 关闭隐藏
	ResetStrikethrough = "\x1B[29m" // 关闭删除线
)
