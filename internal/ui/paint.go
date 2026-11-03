//go:build windows

package ui

import (
	"syscall"

	"keyboard-statistics/internal/stats"
)

// 调色板。GDI 的 COLORREF 是 0x00BBGGRR，统一用 rgb() 生成，绘制代码里不写裸数值。
var (
	colWindowBg    = rgb(0xF5, 0xF5, 0xF5)
	colBarBg       = rgb(0xFF, 0xFF, 0xFF)
	colBorder      = rgb(0xE0, 0xE0, 0xE0)
	colText        = rgb(0x1F, 0x1F, 0x1F)
	colTextDim     = rgb(0x5F, 0x5F, 0x5F)
	colTextOff     = rgb(0xA5, 0xA5, 0xA5)
	colAccent      = rgb(0x2F, 0x6F, 0xEB)
	colAccentHover = rgb(0x25, 0x5D, 0xD0)
	colOnAccent    = rgb(0xFF, 0xFF, 0xFF)
	colCollecting  = rgb(0x1E, 0x8E, 0x3E)
	colPaused      = rgb(0x8A, 0x8A, 0x8A)
	colWarn        = rgb(0xC5, 0x2B, 0x1E)
	colBtnBg       = rgb(0xFF, 0xFF, 0xFF)
	colBtnHover    = rgb(0xEC, 0xF2, 0xFE)
	colDangerBg    = rgb(0xFC, 0xF0, 0xEE)
	colDangerLine  = rgb(0xE8, 0xC4, 0xBE)
	colDangerHover = rgb(0xFA, 0xE3, 0xDF)
	colStatusBg    = rgb(0xEE, 0xEE, 0xEE)
	// TOP 榜行内的比例条：只是背景暗示，要浅到压在上面的黑字依然清楚。
	colOvBar    = rgb(0xEC, 0xF1, 0xFC)
	colOvBarTop = rgb(0xD9, 0xE4, 0xFA)
)

func rgb(r, g, b byte) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

// uiFontFace 优先用微软雅黑 UI；系统没有这个字体时 GDI 会回退到默认中文字体，
// 不会出现方块（iCharSet 已设为 DEFAULT_CHARSET）。
const uiFontFace = "Microsoft YaHei UI"

type fontID int

const (
	fontBody fontID = iota
	fontBold
	fontTitle
	fontSection
	fontSmall
)

// createFonts 按当前 DPI 建好整套字体，绘制时只做 SelectObject。
func (a *App) createFonts() {
	a.fonts = map[fontID]syscall.Handle{
		fontBody:    createFont(-px(14, a.scale), fwNormal, uiFontFace),
		fontBold:    createFont(-px(14, a.scale), fwBold, uiFontFace),
		fontTitle:   createFont(-px(19, a.scale), fwBold, uiFontFace),
		fontSection: createFont(-px(15, a.scale), fwSemiBold, uiFontFace),
		fontSmall:   createFont(-px(12, a.scale), fwNormal, uiFontFace),
	}
}

// canvas 是一块可以画的 DC，外加取字体用的 App。
// 绘制函数只认它，不关心自己画在内存位图上还是窗口上。
type canvas struct {
	hdc syscall.Handle
	a   *App
}

func (cv canvas) fill(r Rect, color uint32) {
	if r.Empty() {
		return
	}
	brush := createSolidBrush(color)
	if brush == 0 {
		return
	}
	fillRect(cv.hdc, r, brush)
	deleteObject(brush)
}

// border 画 1px 边框。用四条细矩形而不是画笔，省掉 SelectObject/DeleteObject 配对。
func (cv canvas) border(r Rect, color uint32) {
	if r.Width() < 2 || r.Height() < 2 {
		return
	}
	cv.fill(Rect{r.Left, r.Top, r.Right, r.Top + 1}, color)
	cv.fill(Rect{r.Left, r.Bottom - 1, r.Right, r.Bottom}, color)
	cv.fill(Rect{r.Left, r.Top, r.Left + 1, r.Bottom}, color)
	cv.fill(Rect{r.Right - 1, r.Top, r.Right, r.Bottom}, color)
}

func (cv canvas) text(s string, r Rect, f fontID, color uint32, flags uint32) {
	if s == "" || r.Empty() {
		return
	}
	var old syscall.Handle
	if font := cv.a.fonts[f]; font != 0 {
		old = selectObject(cv.hdc, font)
	}
	setTextColor(cv.hdc, color)
	drawText(cv.hdc, s, r, flags)
	if old != 0 {
		selectObject(cv.hdc, old)
	}
}

type buttonStyle int

const (
	btnSecondary buttonStyle = iota
	btnPrimary
	btnDanger
)

func (cv canvas) button(r Rect, label string, style buttonStyle, hover bool) {
	switch style {
	case btnPrimary:
		bg := colAccent
		if hover {
			bg = colAccentHover
		}
		cv.fill(r, bg)
		cv.text(label, r, fontBold, colOnAccent, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
	case btnDanger:
		bg := colDangerBg
		if hover {
			bg = colDangerHover
		}
		cv.fill(r, bg)
		cv.border(r, colDangerLine)
		cv.text(label, r, fontBody, colWarn, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
	default:
		bg := colBtnBg
		if hover {
			bg = colBtnHover
		}
		cv.fill(r, bg)
		cv.border(r, colBorder)
		cv.text(label, r, fontBody, colText, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
	}
}

// checkbox 画一个自绘复选框：左侧方框（勾选时填强调色 + 白色对勾），右侧文字。
// 整块 r 都是有效点击区，所以点文字也能切换。
func (cv canvas) checkbox(r Rect, label string, checked, enabled, hover bool) {
	size := r.Height()
	if maxSize := px(18, cv.a.scale); size > maxSize {
		size = maxSize
	}
	if size <= 0 {
		size = px(16, cv.a.scale)
	}
	top := r.Top + (r.Height()-size)/2
	box := Rect{r.Left, top, r.Left + size, top + size}

	if checked {
		cv.fill(box, colAccent)
		cv.text("✓", box, fontSmall, colOnAccent, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
	} else {
		cv.fill(box, colBtnBg)
		borderCol := colBorder
		if enabled && hover {
			borderCol = colAccent
		}
		cv.border(box, borderCol)
	}

	labelColor := colText
	if !enabled {
		labelColor = colTextOff
	}
	labelRect := Rect{box.Right + px(8, cv.a.scale), r.Top, r.Right, r.Bottom}
	cv.text(label, labelRect, fontBody, labelColor, dtVCenter|dtSingleLine|dtNoPrefix|dtEndEllipsis)
}

// paintWindow 是 WM_PAINT 的全部工作：先画到内存位图，再一次性 BitBlt 到窗口，
// 这样窗口不会因为逐块填充而闪烁。
func (a *App) paintWindow(h syscall.Handle) {
	var ps paintStruct
	hdc := beginPaint(h, &ps)
	defer endPaint(h, &ps)

	client := clientRect(h)
	if client.Empty() {
		return
	}

	setBkMode(hdc, transparentBk)

	memDC := createCompatibleDC(hdc)
	bmp := createCompatibleBitmap(hdc, client.Width(), client.Height())
	if memDC == 0 || bmp == 0 {
		if bmp != 0 {
			deleteObject(bmp)
		}
		if memDC != 0 {
			deleteDC(memDC)
		}
		// 双缓冲资源拿不到时退化成直接画：会闪，但总比空白好。
		setBkMode(hdc, transparentBk)
		a.paintAll(canvas{hdc: hdc, a: a})
		return
	}

	old := selectObject(memDC, bmp)
	setBkMode(memDC, transparentBk)

	a.paintAll(canvas{hdc: memDC, a: a})

	// BitBlt 必须在位图仍被选中时做：先 selectObject 还原旧位图再拷，
	// 拷到的就是那块 1×1 的占位位图，窗口会整片空白。
	bitBlt(hdc, client.Width(), client.Height(), memDC)

	selectObject(memDC, old)
	deleteObject(bmp)
	deleteDC(memDC)
}

// paintAll 按当前状态把整个客户区画一遍。
func (a *App) paintAll(cv canvas) {
	l := a.layout()
	cv.fill(l.Client, colWindowBg)

	a.paintToolbar(cv, l)
	a.paintTabBar(cv, l)
	a.paintStatusBar(cv, l)

	switch a.page {
	case pageOverview:
		a.paintOverview(cv, l.Content)
	case pageHeatmap:
		a.paintHeatmap(cv, l.Content)
	case pageSettings:
		a.paintSettings(cv, l.Content)
	}
}

func (a *App) paintToolbar(cv canvas, l Layout) {
	cv.fill(l.Toolbar, colBarBg)
	cv.fill(Rect{l.Toolbar.Left, l.Toolbar.Bottom - 1, l.Toolbar.Right, l.Toolbar.Bottom}, colBorder)

	cv.text(windowTitle, l.Title, fontTitle, colText, dtVCenter|dtSingleLine|dtNoPrefix)

	scopes := stats.AllScopes()
	for i, r := range l.Scopes {
		if i >= len(scopes) {
			break
		}
		label := scopes[i].Label()
		switch {
		case scopes[i] == a.scope:
			cv.fill(r, colAccent)
			cv.text(label, r, fontBold, colOnAccent, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
		case a.hover.kind == hoverScope && a.hover.index == i:
			cv.fill(r, colBtnHover)
			cv.text(label, r, fontBody, colAccent, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
		default:
			cv.text(label, r, fontBody, colTextDim, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
		}
	}

	statusColor := colPaused
	if a.collecting {
		statusColor = colCollecting
	}
	cv.text(statusText(a.collecting), l.Status, fontBody, statusColor, dtRight|dtVCenter|dtSingleLine|dtNoPrefix)
}

func (a *App) paintTabBar(cv canvas, l Layout) {
	cv.fill(l.TabBar, colBarBg)
	cv.fill(Rect{l.TabBar.Left, l.TabBar.Bottom - 1, l.TabBar.Right, l.TabBar.Bottom}, colBorder)

	for i, r := range l.Tabs {
		if i >= len(tabTitles) {
			break
		}
		selected := pageID(i) == a.page
		hovered := a.hover.kind == hoverTab && a.hover.index == i

		switch {
		case selected:
			cv.text(tabTitles[i], r, fontBold, colAccent, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
			underlineH := px(3, a.scale)
			cv.fill(Rect{r.Left + px(6, a.scale), r.Bottom - underlineH, r.Right - px(6, a.scale), r.Bottom}, colAccent)
		case hovered:
			cv.text(tabTitles[i], r, fontBody, colAccent, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
		default:
			cv.text(tabTitles[i], r, fontBody, colTextDim, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
		}
	}
}

func (a *App) paintStatusBar(cv canvas, l Layout) {
	cv.fill(l.StatusBar, colStatusBg)
	cv.fill(Rect{l.StatusBar.Left, l.StatusBar.Top, l.StatusBar.Right, l.StatusBar.Top + 1}, colBorder)

	text := "正在读取数据…"
	color := colTextDim
	if a.snapErr != "" {
		text = "读取数据失败：" + a.snapErr
		color = colWarn
	} else if !a.lastRefresh.IsZero() {
		text = refreshSummary(a.snap.From, a.snap.To, a.snap.Days, a.lastRefresh)
	}

	padded := Rect{
		l.StatusBar.Left + px(basePadX, a.scale), l.StatusBar.Top,
		l.StatusBar.Right - px(basePadX, a.scale), l.StatusBar.Bottom,
	}
	cv.text(text, padded, fontSmall, color, dtVCenter|dtSingleLine|dtNoPrefix|dtEndEllipsis)
}

// paintPlaceholder 是还没实现的页面的占位：一句话说明现在什么都没有。
func (a *App) paintPlaceholder(cv canvas, content Rect, title, hint string) {
	mid := content.Top + content.Height()/2

	titleRect := Rect{content.Left, mid - px(26, a.scale), content.Right, mid + px(6, a.scale)}
	cv.text(title, titleRect, fontTitle, colTextDim, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)

	hintRect := Rect{
		content.Left + px(basePadX*3, a.scale), titleRect.Bottom + px(4, a.scale),
		content.Right - px(basePadX*3, a.scale), titleRect.Bottom + px(52, a.scale),
	}
	cv.text(hint, hintRect, fontSmall, colTextOff, dtCenter|dtWordBreak|dtNoPrefix)
}
