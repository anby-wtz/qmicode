package gui

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"

	appconfig "qmicode-go/app/config"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/fontscan"
	"go.yaml.in/yaml/v3"
)

const (
	guiConfigFileName = "config_gui.yaml"
	defaultFontSize   = float32(16)
	minFontSize       = float32(10)
	maxFontSize       = float32(32)

	fontOptionDefault = "系统默认字体"
	themeModeLight    = "light"
	themeModeDark     = "dark"
	themeModeSystem   = "system"
)

type guiSettings struct {
	FontFamily string  `yaml:"font_family,omitempty"`
	FontSize   float32 `yaml:"font_size"`
	ThemeMode  string  `yaml:"theme_mode,omitempty"`
	Language   string  `yaml:"language,omitempty"`
}

func defaultGUISettings() guiSettings {
	return guiSettings{
		FontSize:  defaultFontSize,
		ThemeMode: themeModeLight,
		Language:  languageChinese,
	}
}

func guiSettingsPath() string {
	return filepath.Join(appconfig.DefaultConfig.WorkDir, guiConfigFileName)
}

func loadGUISettings() (guiSettings, error) {
	settings := defaultGUISettings()
	data, err := os.ReadFile(guiSettingsPath())
	if os.IsNotExist(err) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return defaultGUISettings(), fmt.Errorf("解析 GUI 设置失败：%w", err)
	}
	if settings.FontSize == 0 {
		settings.FontSize = defaultFontSize
	}
	if math.IsNaN(float64(settings.FontSize)) || math.IsInf(float64(settings.FontSize), 0) || settings.FontSize < minFontSize || settings.FontSize > maxFontSize {
		return defaultGUISettings(), fmt.Errorf("GUI 字号必须在 %.0f 到 %.0f 之间", minFontSize, maxFontSize)
	}
	if settings.ThemeMode == "" {
		settings.ThemeMode = themeModeLight
	}
	if !validThemeMode(settings.ThemeMode) {
		return defaultGUISettings(), fmt.Errorf("未知 GUI 主题模式 %q", settings.ThemeMode)
	}
	if settings.Language == "" {
		settings.Language = languageChinese
	}
	if !validLanguage(settings.Language) {
		return defaultGUISettings(), fmt.Errorf("未知 GUI 语言 %q", settings.Language)
	}
	return settings, nil
}

func saveGUISettings(settings guiSettings) error {
	if math.IsNaN(float64(settings.FontSize)) || math.IsInf(float64(settings.FontSize), 0) || settings.FontSize < minFontSize || settings.FontSize > maxFontSize {
		return fmt.Errorf("GUI 字号必须在 %.0f 到 %.0f 之间", minFontSize, maxFontSize)
	}
	if !validThemeMode(settings.ThemeMode) {
		return fmt.Errorf("未知 GUI 主题模式 %q", settings.ThemeMode)
	}
	if !validLanguage(settings.Language) {
		return fmt.Errorf("未知 GUI 语言 %q", settings.Language)
	}
	data, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(appconfig.DefaultConfig.WorkDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(guiSettingsPath(), data, 0644)
}

func validThemeMode(mode string) bool {
	return mode == themeModeLight || mode == themeModeDark || mode == themeModeSystem
}

func validLanguage(language string) bool {
	return language == languageChinese || language == languageEnglish
}

type appearanceTheme struct {
	base     fyne.Theme
	fonts    map[fyne.TextStyle]fyne.Resource
	fontSize float32
	mode     string
}

func (t *appearanceTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch t.mode {
	case themeModeLight:
		variant = theme.VariantLight
	case themeModeDark:
		variant = theme.VariantDark
	}
	return t.base.Color(name, variant)
}

func (t *appearanceTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Symbol {
		return t.base.Font(style)
	}
	if resource, ok := t.fonts[style]; ok && resource != nil {
		return resource
	}
	if regular, ok := t.fonts[fyne.TextStyle{}]; ok && regular != nil {
		return regular
	}
	return t.base.Font(style)
}

func (t *appearanceTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *appearanceTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText, theme.SizeNameHeadingText, theme.SizeNameSubHeadingText, theme.SizeNameCaptionText:
		baseTextSize := t.base.Size(theme.SizeNameText)
		if baseTextSize > 0 {
			return t.base.Size(name) * t.fontSize / baseTextSize
		}
	}
	return t.base.Size(name)
}

type installedFont struct {
	Family          string
	RegularPath     string
	BoldPath        string
	ItalicPath      string
	BoldItalicPath  string
	regularScore    float64
	boldScore       float64
	italicScore     float64
	boldItalicScore float64
}

type silentFontLogger struct{}

func (silentFontLogger) Printf(string, ...interface{}) {}

var fontCatalogState struct {
	once     sync.Once
	fonts    []installedFont
	byFamily map[string]installedFont
	err      error
}

func installedSystemFonts() ([]installedFont, error) {
	fontCatalogState.once.Do(func() {
		footprints, err := fontscan.SystemFonts(silentFontLogger{}, "")
		if err != nil {
			fontCatalogState.err = err
			return
		}

		records := make(map[string]*installedFont)
		for _, footprint := range footprints {
			family := footprint.Family
			ext := strings.ToLower(filepath.Ext(footprint.Location.File))
			if family == "" || (ext != ".ttf" && ext != ".otf") {
				continue
			}
			record := records[family]
			if record == nil {
				record = &installedFont{
					Family:          family,
					regularScore:    math.Inf(1),
					boldScore:       math.Inf(1),
					italicScore:     math.Inf(1),
					boldItalicScore: math.Inf(1),
				}
				records[family] = record
			}

			path := footprint.Location.File
			aspect := footprint.Aspect
			if aspect.Style == font.StyleItalic {
				if aspect.Weight >= font.WeightSemibold {
					setFontCandidate(&record.BoldItalicPath, &record.boldItalicScore, path, aspect, font.StyleItalic, font.WeightBold)
				} else {
					setFontCandidate(&record.ItalicPath, &record.italicScore, path, aspect, font.StyleItalic, font.WeightNormal)
				}
			} else if aspect.Weight >= font.WeightSemibold {
				setFontCandidate(&record.BoldPath, &record.boldScore, path, aspect, font.StyleNormal, font.WeightBold)
			} else {
				setFontCandidate(&record.RegularPath, &record.regularScore, path, aspect, font.StyleNormal, font.WeightNormal)
			}
		}

		fontCatalogState.byFamily = make(map[string]installedFont)
		for _, record := range records {
			if record.RegularPath == "" {
				continue
			}
			fontCatalogState.fonts = append(fontCatalogState.fonts, *record)
			fontCatalogState.byFamily[record.Family] = *record
		}
		sort.Slice(fontCatalogState.fonts, func(i, j int) bool {
			return fontCatalogState.fonts[i].Family < fontCatalogState.fonts[j].Family
		})
	})
	return fontCatalogState.fonts, fontCatalogState.err
}

func setFontCandidate(path *string, currentScore *float64, candidatePath string, aspect font.Aspect, style font.Style, weight font.Weight) {
	score := math.Abs(float64(aspect.Weight - weight))
	if aspect.Style != style {
		score += 10000
	}
	if score < *currentScore {
		*path = candidatePath
		*currentScore = score
	}
}

func buildAppearanceTheme(settings guiSettings) (fyne.Theme, error) {
	if math.IsNaN(float64(settings.FontSize)) || math.IsInf(float64(settings.FontSize), 0) || settings.FontSize < minFontSize || settings.FontSize > maxFontSize {
		return nil, fmt.Errorf("GUI 字号必须在 %.0f 到 %.0f 之间", minFontSize, maxFontSize)
	}
	if !validThemeMode(settings.ThemeMode) {
		return nil, fmt.Errorf("未知 GUI 主题模式 %q", settings.ThemeMode)
	}

	base := theme.DefaultTheme()
	fonts := make(map[fyne.TextStyle]fyne.Resource)
	if settings.FontFamily != "" {
		if _, err := installedSystemFonts(); err != nil {
			return nil, fmt.Errorf("读取系统字体列表失败：%w", err)
		}
		selected, ok := fontCatalogState.byFamily[settings.FontFamily]
		if !ok {
			return nil, fmt.Errorf("系统中未找到字体 %q", settings.FontFamily)
		}
		regular, err := loadFontResource(selected.RegularPath)
		if err != nil {
			return nil, err
		}
		fonts[fyne.TextStyle{}] = regular
		for _, item := range []struct {
			style fyne.TextStyle
			path  string
		}{
			{style: fyne.TextStyle{Bold: true}, path: selected.BoldPath},
			{style: fyne.TextStyle{Italic: true}, path: selected.ItalicPath},
			{style: fyne.TextStyle{Bold: true, Italic: true}, path: selected.BoldItalicPath},
		} {
			fonts[item.style] = regular
			if item.path == "" {
				continue
			}
			if resource, err := loadFontResource(item.path); err == nil {
				fonts[item.style] = resource
			}
		}
	}

	return &appearanceTheme{
		base:     base,
		fonts:    fonts,
		fontSize: settings.FontSize,
		mode:     settings.ThemeMode,
	}, nil
}

func loadFontResource(path string) (fyne.Resource, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".ttf" && ext != ".otf" {
		return nil, fmt.Errorf("系统字体文件类型 %q 目前不受支持", ext)
	}
	resource, err := fyne.LoadResourceFromPath(path)
	if err != nil {
		return nil, fmt.Errorf("读取系统字体文件 %q 失败：%w", path, err)
	}
	return resource, nil
}

func displayFontFamily(family string) string {
	words := strings.Fields(family)
	for i, word := range words {
		runes := []rune(word)
		if len(runes) == 0 {
			continue
		}
		if len(runes) <= 3 {
			words[i] = strings.ToUpper(word)
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

func themeModeOption(mode string, g *desktopUI) string {
	switch mode {
	case themeModeDark:
		return g.tr("深色")
	case themeModeSystem:
		return g.tr("跟随系统")
	default:
		return g.tr("浅色")
	}
}

func themeModeValue(option string, g *desktopUI) string {
	switch option {
	case g.tr("深色"):
		return themeModeDark
	case g.tr("跟随系统"):
		return themeModeSystem
	default:
		return themeModeLight
	}
}

func (g *desktopUI) openSettings() {
	if g.settingsWindow != nil {
		g.settingsWindow.RequestFocus()
		return
	}
	settingsWindow := g.app.NewWindow(g.tr("GUI 设置"))
	g.settingsWindow = settingsWindow
	settingsWindow.SetOnClosed(func() {
		if g.settingsWindow == settingsWindow {
			g.settingsWindow = nil
		}
	})

	fontFamily := g.guiSettings.FontFamily
	defaultFontOption := g.tr(fontOptionDefault)
	fontOptions := make(map[string]string)
	fontSelect := widget.NewSelect(nil, func(option string) {
		fontFamily = fontOptions[option]
	})
	fontSelect.Disable()
	fontStatus := widget.NewLabel(g.tr("正在读取系统已安装字体列表…"))

	themeMode := g.guiSettings.ThemeMode
	themeSelect := widget.NewSelect([]string{g.tr("浅色"), g.tr("深色"), g.tr("跟随系统")}, func(option string) {
		themeMode = themeModeValue(option, g)
	})
	themeSelect.SetSelected(themeModeOption(themeMode, g))
	language := g.guiSettings.Language
	languageSelect := widget.NewSelect([]string{"简体中文", "English"}, func(option string) {
		language = languageValue(option)
	})
	languageSelect.SetSelected(languageDisplay(language))

	size := widget.NewSlider(float64(minFontSize), float64(maxFontSize))
	size.Step = 1
	size.Value = float64(g.guiSettings.FontSize)
	sizeLabel := widget.NewLabel(fmt.Sprintf("%.0f", size.Value))
	size.OnChanged = func(value float64) {
		sizeLabel.SetText(fmt.Sprintf("%.0f", value))
	}

	form := widget.NewForm(
		widget.NewFormItem(g.tr("系统字体"), container.NewVBox(fontSelect, fontStatus)),
		widget.NewFormItem(g.tr("字号（10–32）"), container.NewVBox(size, sizeLabel)),
		widget.NewFormItem(g.tr("背景主题"), themeSelect),
		widget.NewFormItem(g.tr("界面语言"), languageSelect),
	)

	cancel := widget.NewButton(g.tr("取消"), settingsWindow.Close)
	apply := widget.NewButton(g.tr("保存并应用"), func() {
		settings := guiSettings{
			FontFamily: fontFamily,
			FontSize:   float32(math.Round(size.Value)),
			ThemeMode:  themeMode,
			Language:   language,
		}
		if fontFamily != "" {
			if _, err := installedSystemFonts(); err != nil {
				g.showError(fmt.Errorf("读取系统字体列表失败：%w", err), settingsWindow)
				return
			}
			if _, ok := fontCatalogState.byFamily[fontFamily]; !ok {
				g.showError(fmt.Errorf("系统中未找到字体 %q", fontFamily), settingsWindow)
				return
			}
		}
		appearance, err := buildAppearanceTheme(settings)
		if err != nil {
			g.showError(err, settingsWindow)
			return
		}
		if err := saveGUISettings(settings); err != nil {
			g.showError(fmt.Errorf("保存 %s 失败：%w", guiSettingsPath(), err), settingsWindow)
			return
		}
		g.app.Settings().SetTheme(appearance)
		g.guiSettings = settings
		g.applyLanguage()
		g.setStatus("GUI 设置已保存并应用")
		settingsWindow.Close()
	})
	apply.Disable()
	apply.Importance = widget.HighImportance

	settingsWindow.SetContent(container.NewBorder(
		widget.NewLabel(g.tr("选择系统已安装字体；设置会立即应用并保存供下次启动使用。")),
		container.NewHBox(cancel, apply),
		nil,
		nil,
		form,
	))
	settingsWindow.Resize(fyne.NewSize(640, 400))
	settingsWindow.Show()

	go func() {
		fonts, err := installedSystemFonts()
		fyne.Do(func() {
			if g.settingsWindow != settingsWindow {
				return
			}
			fontOptions = map[string]string{defaultFontOption: ""}
			options := []string{defaultFontOption}
			selectedOption := defaultFontOption
			foundSelected := fontFamily == ""
			for _, installed := range fonts {
				option := displayFontFamily(installed.Family)
				fontOptions[option] = installed.Family
				options = append(options, option)
				if installed.Family == fontFamily {
					selectedOption = option
					foundSelected = true
				}
			}
			if err != nil {
				fontFamily = ""
				fontStatus.SetText(g.trf("读取字体列表失败：%s", g.localizeError(err).Error()))
			} else if !foundSelected {
				fontFamily = ""
				fontStatus.SetText(g.tr("此前保存的字体当前不可用，已改选系统默认字体。"))
			} else {
				fontStatus.SetText(g.trf("找到 %d 个可用的系统字体", len(fonts)))
			}
			fontSelect.SetOptions(options)
			fontSelect.SetSelected(selectedOption)
			fontSelect.Enable()
			apply.Enable()
		})
	}()
}
