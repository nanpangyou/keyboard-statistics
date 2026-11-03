package ui

import "math"

// Rect 描述客户区坐标系里的矩形。
//
// 字段顺序与 Win32 的 RECT 一致（左/上/右/下），右下为开区间——这样可以直接把
// &Rect 交给 FillRect / DrawText 之类的 GDI 函数，不必再转一次坐标。
type Rect struct {
	Left, Top, Right, Bottom int32
}

// Width 返回矩形宽度（可能为负，见 Empty）。
func (r Rect) Width() int32 { return r.Right - r.Left }

// Height 返回矩形高度（可能为负，见 Empty）。
func (r Rect) Height() int32 { return r.Bottom - r.Top }

// Empty 报告矩形是否没有面积。布局在窄窗口里压缩控件时会出现这种矩形，
// 绘制与命中测试都要先跳过它。
func (r Rect) Empty() bool { return r.Right <= r.Left || r.Bottom <= r.Top }

// Contains 报告点是否落在矩形内。右下边界算外面，避免相邻控件在边界上重复命中。
func (r Rect) Contains(x, y int32) bool {
	return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

// 下面这些尺寸是 96 DPI 下的基准值，实际使用前统一乘 DPI 比例。
// 界面只有文字、按钮和图表，尺寸集中在这里就不必到绘制代码里找魔法数。
const (
	baseToolbarH = 56
	baseTabBarH  = 40
	baseStatusH  = 24

	basePadX   = 20
	baseTitleW = 150

	baseScopeW   = 92
	baseScopeH   = 32
	baseScopeGap = 8

	baseTabW = 96

	baseStatusW = 120

	baseSetPad         = 20
	baseSectionGap     = 20
	baseSectionTitleH  = 26
	baseButtonH        = 32
	baseButtonW        = 132
	baseWideButtonW    = 152
	baseButtonGap      = 12
	baseCheckH         = 24
	baseHintH          = 20
	baseHintGap        = 8
	baseAboutLineH     = 22
	baseAboutGap       = 12
	baseContentTopGap  = 12
	baseStaleHintGap   = 4
	baseSettingsBottom = 16
)

// px 把 96 DPI 基准值按缩放换算成像素。scale <= 0 时按 100% 处理，
// 免得调用方漏传 DPI 时算出零宽控件。
func px(base int32, scale float64) int32 {
	if scale <= 0 {
		scale = 1
	}
	return int32(math.Round(float64(base) * scale))
}

// Layout 是「窗口客户区 → 各区域与可点击控件」的纯计算结果。
//
// 把它独立出来是因为这是界面里最容易被改坏、又最好测的部分：
// 换分辨率、加一个时间范围、加一个 Tab 都只影响这里。
type Layout struct {
	Client    Rect
	Toolbar   Rect
	TabBar    Rect
	Content   Rect
	StatusBar Rect

	Title  Rect
	Scopes []Rect
	Status Rect
	Tabs   []Rect
}

// ComputeLayout 计算三段式布局：顶部工具栏、标签栏、内容区、底部状态栏。
//
// 一律按「先算栏、再算栏内控件」的顺序，保证任何尺寸下区域之间不会互相压住。
func ComputeLayout(client Rect, scale float64, scopeCount, tabCount int) Layout {
	if scopeCount < 0 {
		scopeCount = 0
	}
	if tabCount < 0 {
		tabCount = 0
	}

	toolbarH := px(baseToolbarH, scale)
	tabBarH := px(baseTabBarH, scale)
	statusH := px(baseStatusH, scale)

	l := Layout{Client: client}

	l.Toolbar = Rect{client.Left, client.Top, client.Right, client.Top + toolbarH}
	l.TabBar = Rect{client.Left, l.Toolbar.Bottom, client.Right, l.Toolbar.Bottom + tabBarH}
	l.StatusBar = Rect{client.Left, client.Bottom - statusH, client.Right, client.Bottom}
	l.Content = Rect{client.Left, l.TabBar.Bottom, client.Right, l.StatusBar.Top}

	padX := px(basePadX, scale)

	l.Title = Rect{
		Left:   client.Left + padX,
		Top:    l.Toolbar.Top,
		Right:  client.Left + padX + px(baseTitleW, scale),
		Bottom: l.Toolbar.Bottom,
	}
	l.Status = Rect{
		Left:   client.Right - padX - px(baseStatusW, scale),
		Top:    l.Toolbar.Top,
		Right:  client.Right - padX,
		Bottom: l.Toolbar.Bottom,
	}

	// 时间范围按钮整体居中：它比标题和状态文字更重要，居中最好点。
	scopeW := px(baseScopeW, scale)
	scopeH := px(baseScopeH, scale)
	gap := px(baseScopeGap, scale)
	if scopeCount > 0 {
		total := int32(scopeCount)*scopeW + int32(scopeCount-1)*gap
		start := client.Left + (client.Width()-total)/2
		if minStart := client.Left + padX; start < minStart {
			start = minStart
		}
		if maxStart := client.Right - padX - total; maxStart >= client.Left+padX && start > maxStart {
			start = maxStart
		}
		top := l.Toolbar.Top + (toolbarH-scopeH)/2

		l.Scopes = make([]Rect, scopeCount)
		for i := 0; i < scopeCount; i++ {
			left := start + int32(i)*(scopeW+gap)
			l.Scopes[i] = Rect{left, top, left + scopeW, top + scopeH}
		}
	}

	// 标签从左边距开始，高度占满整条标签栏，命中区域大一点更好点。
	if tabCount > 0 {
		tabW := px(baseTabW, scale)
		start := client.Left + padX

		l.Tabs = make([]Rect, tabCount)
		for i := 0; i < tabCount; i++ {
			left := start + int32(i)*tabW
			l.Tabs[i] = Rect{left, l.TabBar.Top, left + tabW, l.TabBar.Bottom}
		}
	}

	return l
}

// HitScope 返回命中的时间范围按钮下标。
func (l Layout) HitScope(x, y int32) (int, bool) {
	for i, r := range l.Scopes {
		if r.Contains(x, y) {
			return i, true
		}
	}
	return 0, false
}

// HitTab 返回命中的标签下标。
func (l Layout) HitTab(x, y int32) (int, bool) {
	for i, r := range l.Tabs {
		if r.Contains(x, y) {
			return i, true
		}
	}
	return 0, false
}

// SettingsHit 标识设置页上被点中的控件。
type SettingsHit int

const (
	SettingsHitNone SettingsHit = iota
	SettingsHitCollect
	SettingsHitAutostart
	SettingsHitExport
	SettingsHitClear
	SettingsHitOpenDataDir
	SettingsHitChangeDataDir
)

// SettingsLayout 是设置页的控件位置。
//
// 设置页是自绘的，没有控件句柄，所有点击都靠这里的矩形做命中测试，
// 因此这份布局同时也是「设置页有哪些可点区域」的唯一清单。
type SettingsLayout struct {
	Content Rect

	SectionCollect Rect
	CollectBtn     Rect
	CollectHint    Rect

	SectionAutostart Rect
	AutostartBox     Rect
	StaleHint        Rect

	SectionData Rect
	ExportBtn   Rect
	ClearBtn    Rect
	DataHint    Rect

	SectionAbout Rect
	OpenDirBtn   Rect
	ChangeDirBtn Rect

	AboutTop int32
	LineH    int32
}

// ComputeSettingsLayout 自上而下排布设置页：采集 → 开机自启 → 数据 → 关于。
func ComputeSettingsLayout(content Rect, scale float64) SettingsLayout {
	pad := px(baseSetPad, scale)
	titleH := px(baseSectionTitleH, scale)
	btnH := px(baseButtonH, scale)
	btnW := px(baseButtonW, scale)
	hintH := px(baseHintH, scale)
	right := content.Right - pad
	x := content.Left + pad

	s := SettingsLayout{
		Content: content,
		LineH:   px(baseAboutLineH, scale),
	}

	y := content.Top + px(baseContentTopGap, scale)

	s.SectionCollect = Rect{x, y, right, y + titleH}
	y += titleH
	s.CollectBtn = Rect{x, y, x + btnW, y + btnH}
	y += btnH + px(baseHintGap, scale)
	s.CollectHint = Rect{x, y, right, y + hintH}
	y += hintH + px(baseSectionGap, scale)

	s.SectionAutostart = Rect{x, y, right, y + titleH}
	y += titleH
	s.AutostartBox = Rect{x, y, right, y + px(baseCheckH, scale)}
	y += px(baseCheckH, scale) + px(baseStaleHintGap, scale)
	s.StaleHint = Rect{x, y, right, y + hintH}
	y += hintH + px(baseSectionGap, scale)

	s.SectionData = Rect{x, y, right, y + titleH}
	y += titleH
	s.ExportBtn = Rect{x, y, x + btnW, y + btnH}
	s.ClearBtn = Rect{
		x + btnW + px(baseButtonGap, scale), y,
		x + btnW + px(baseButtonGap, scale) + px(baseWideButtonW, scale), y + btnH,
	}
	y += btnH + px(baseHintGap, scale)
	s.DataHint = Rect{x, y, right, y + hintH}
	y += hintH + px(baseSectionGap, scale)

	s.SectionAbout = Rect{x, y, right, y + titleH}
	y += titleH
	s.OpenDirBtn = Rect{x, y, x + btnW, y + btnH}
	s.ChangeDirBtn = Rect{
		x + btnW + px(baseButtonGap, scale), y,
		x + btnW + px(baseButtonGap, scale) + px(baseWideButtonW, scale), y + btnH,
	}
	y += btnH + px(baseAboutGap, scale)
	s.AboutTop = y

	return s
}

// Hit 返回设置页上命中的控件。
func (s SettingsLayout) Hit(x, y int32) SettingsHit {
	switch {
	case s.CollectBtn.Contains(x, y):
		return SettingsHitCollect
	case s.AutostartBox.Contains(x, y):
		return SettingsHitAutostart
	case s.ExportBtn.Contains(x, y):
		return SettingsHitExport
	case s.ClearBtn.Contains(x, y):
		return SettingsHitClear
	case s.OpenDirBtn.Contains(x, y):
		return SettingsHitOpenDataDir
	case s.ChangeDirBtn.Contains(x, y):
		return SettingsHitChangeDataDir
	}
	return SettingsHitNone
}
