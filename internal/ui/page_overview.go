//go:build windows

package ui

import "syscall"

// 概览页的绘制：顶部四张指标卡 → 两个 TOP 10 列表 → 底部 24 小时时段分布。
//
// 所有坐标都来自 ComputeOverviewLayout / overviewSection / overviewHourChart
// （见 overview_layout.go），本文件只按算好的矩形填色填字，不做位置计算。
// 口径（占比分母、时段归一化、空状态判定）同样全在纯函数里，这里不重新算。

// 概览页专用的两个色值。其余颜色复用 paint.go 的调色板，
// 放这里而不是往 paint.go 里加，避免和同时在改别的页面的代码冲突。
var (
	colOvCard     = rgb(0xFF, 0xFF, 0xFF)
	colOvCardLine = rgb(0xE6, 0xE6, 0xE6)
	colOvBarIdle  = rgb(0xDE, 0xDE, 0xDE)
)

func (a *App) paintOverview(cv canvas, content Rect) {
	l := ComputeOverviewLayout(content, a.scale)
	if l.Content.Empty() {
		return
	}

	snap := a.snap
	m := overviewMetrics{
		TotalKeys:     snap.TotalKeys,
		ActiveSeconds: snap.ActiveSeconds,
		DailyAverage:  snap.DailyAverage,
		PeakCount:     snap.PeakCount,
		PeakDay:       snap.PeakDay,
		Days:          snap.Days,
	}
	a.paintOverviewCards(cv, l.Cards, overviewCardSpecs(m))

	// 一点数据都没有时只画卡片（数值全是「—」）加一句友好文案，
	// 不画三块空图表：一堆 0 和空柱子只会让人以为统计坏了。
	if overviewEmpty(snap.TotalKeys) {
		a.paintOverviewEmpty(cv, l.Body)
		return
	}

	a.paintOverviewList(cv, l.Keys, "按键 TOP 10",
		overviewKeyRows(snap.TopKeys, snap.TotalKeys),
		"还没有按键数据", "这个范围里还没有记录到按键。")

	// 组合占比的分母是组合键总次数，不是总按键数（docs/metrics.md §4）。
	comboEmptyTitle, comboEmptyHint := overviewComboEmptyText()
	a.paintOverviewList(cv, l.Combos, "快捷键 TOP 10",
		overviewComboRows(snap.TopCombos, comboTotal(snap.ComboCounts)),
		comboEmptyTitle, comboEmptyHint)

	a.paintOverviewHours(cv, l.Hours, snap.Hourly)
}

// paintOverviewCards 画一行四张等宽指标卡：灰字标题、大号数字、灰色副标题。
func (a *App) paintOverviewCards(cv canvas, cards [4]Rect, specs [4]cardSpec) {
	// 大号数字比 paint.go 里现成的 fontID 都大，又不想为此改 paint.go 的字体表，
	// 就在本地按当前 DPI 现建一个，画完释放：一次重绘多一次 CreateFontW，
	// 比把句柄挂在 App 上却没人负责删除要安全。
	valueFont := createFont(-px(baseOvValueSize, a.scale), fwBold, uiFontFace)
	if valueFont != 0 {
		defer deleteObject(valueFont)
	}

	padX := px(baseOvCardPadX, a.scale)
	padY := px(baseOvCardPadY, a.scale)

	for i, spec := range specs {
		r := cards[i]
		if r.Empty() {
			continue
		}
		cv.fill(r, colOvCard)
		cv.border(r, colOvCardLine)

		inner := Rect{r.Left + padX, r.Top + padY, r.Right - padX, r.Bottom - padY}
		if inner.Empty() {
			continue
		}

		titleRect := Rect{inner.Left, inner.Top, inner.Right, inner.Top + px(baseOvCardTitleH, a.scale)}
		cv.text(spec.Title, titleRect, fontSmall, colTextDim, dtVCenter|dtSingleLine|dtNoPrefix)

		valueRect := Rect{inner.Left, titleRect.Bottom, inner.Right, titleRect.Bottom + px(baseOvCardValueH, a.scale)}
		cv.overviewText(spec.Value, valueRect, valueFont, colText,
			dtVCenter|dtSingleLine|dtNoPrefix|dtEndEllipsis)

		subRect := Rect{inner.Left, valueRect.Bottom, inner.Right, inner.Bottom}
		cv.text(spec.Sub, subRect, fontSmall, colTextOff, dtVCenter|dtSingleLine|dtNoPrefix|dtEndEllipsis)
	}
}

// paintOverviewList 画一块 TOP 榜：标题 + 可选分隔线 + 若干行；没有行时给提示文案。
func (a *App) paintOverviewList(cv canvas, area Rect, title string, rows []overviewRow, emptyTitle, emptyHint string) {
	titleRect, body := overviewSection(area, a.scale)
	if titleRect.Empty() || body.Empty() {
		return
	}

	cv.text(title, titleRect, fontSection, colText, dtVCenter|dtSingleLine|dtNoPrefix)
	cv.fill(Rect{titleRect.Left, titleRect.Bottom - 1, titleRect.Right, titleRect.Bottom}, colBorder)

	if len(rows) == 0 {
		a.paintOverviewHint(cv, body, emptyTitle, emptyHint)
		return
	}

	inset := px(baseOvRowInset, a.scale)
	rowsArea := Rect{body.Left, body.Top + inset, body.Right, body.Bottom - inset}
	rects := overviewRowRects(rowsArea, len(rows))

	// 榜首次数是比例条的分母：条宽 = 本行 / 榜首，扫一眼就知道差距有多大。
	var topCount int64
	for _, row := range rows {
		if row.Raw > topCount {
			topCount = row.Raw
		}
	}

	for i, row := range rows {
		r := rects[i]
		if r.Empty() {
			continue
		}

		rankRect, nameRect, countRect, pctRect := overviewRowCells(r, a.scale)

		// 第一名用强调色 + 粗体，扫一眼就知道最常用的键是哪个。
		rankColor, nameColor, nameFont := colTextOff, colText, fontBody
		barColor := colOvBar
		if i == 0 {
			rankColor, nameFont = colAccent, fontBold
			barColor = colOvBarTop
		}

		// 条先画、字后压：文字始终是主角，条只做背景暗示。
		// 条只铺键名那一列——铺到数字下面会让「900」看起来像是被高亮了。
		if !nameRect.Empty() {
			if bar := overviewRowBar(nameRect, row.Raw, topCount, a.scale); !bar.Empty() {
				cv.fill(bar, barColor)
			}
		}

		cv.text(row.Rank, rankRect, fontSmall, rankColor, dtRight|dtVCenter|dtSingleLine|dtNoPrefix)
		cv.text(row.Name, nameRect, nameFont, nameColor, dtVCenter|dtSingleLine|dtNoPrefix|dtEndEllipsis)
		cv.text(row.Count, countRect, fontBody, colText, dtRight|dtVCenter|dtSingleLine|dtNoPrefix)
		cv.text(row.Percent, pctRect, fontSmall, colTextDim, dtRight|dtVCenter|dtSingleLine|dtNoPrefix)
	}
}

// paintOverviewHint 在区块中央画「标题 + 说明」两句空状态提示。
func (a *App) paintOverviewHint(cv canvas, area Rect, title, hint string) {
	if area.Empty() {
		return
	}
	mid := area.Top + area.Height()/2

	titleRect := Rect{area.Left, mid - px(20, a.scale), area.Right, mid - px(2, a.scale)}
	cv.text(title, titleRect, fontBody, colTextDim, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)

	hintRect := Rect{
		area.Left + px(baseOvPad, a.scale), mid,
		area.Right - px(baseOvPad, a.scale), mid + px(36, a.scale),
	}
	cv.text(hint, hintRect, fontSmall, colTextOff, dtCenter|dtWordBreak|dtNoPrefix)
}

// paintOverviewEmpty 是整页空状态：一句问候 + 一句说明这里将显示什么。
func (a *App) paintOverviewEmpty(cv canvas, area Rect) {
	if area.Empty() {
		return
	}
	title, hint := overviewEmptyText()
	mid := area.Top + area.Height()/2

	titleRect := Rect{area.Left, mid - px(32, a.scale), area.Right, mid - px(6, a.scale)}
	cv.text(title, titleRect, fontTitle, colTextDim, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)

	hintRect := Rect{
		area.Left + px(baseOvPad, a.scale), mid,
		area.Right - px(baseOvPad, a.scale), mid + px(44, a.scale),
	}
	cv.text(hint, hintRect, fontSmall, colTextOff, dtCenter|dtWordBreak|dtNoPrefix)
}

// paintOverviewHours 画 24 小时时段柱状图：24 根柱 + 每 3 小时一个刻度。
func (a *App) paintOverviewHours(cv canvas, area Rect, hourly [24]int64) {
	title, bars, ticks := overviewHourChart(area, a.scale)
	if title.Empty() || bars.Empty() {
		return
	}

	cv.text("24 小时时段分布", title, fontSection, colText, dtVCenter|dtSingleLine|dtNoPrefix)

	max := hourlyMax(hourly)
	if max <= 0 {
		// max=0 是唯一可能出现除零的地方，这里直接给空状态，不画 24 根 0 高的柱子。
		cv.text(overviewHourEmptyText(), bars, fontSmall, colTextOff, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
		return
	}

	// 纵轴只在顶部标一次最大值，不画密网格线（docs/ui-plan.md §6.2）。
	labelH := px(baseOvMaxLabelH, a.scale)
	cv.text(formatKeyCount(max)+" 次/时",
		Rect{bars.Left, bars.Top, bars.Right, bars.Top + labelH},
		fontSmall, colTextOff, dtRight|dtVCenter|dtSingleLine|dtNoPrefix)

	plot := Rect{bars.Left, bars.Top + labelH, bars.Right, bars.Bottom}
	if plot.Empty() {
		return
	}

	// 基线：柱子的底边都压在它上面，视觉上有个「地面」。
	cv.fill(Rect{plot.Left, plot.Bottom - 1, plot.Right, plot.Bottom}, colBorder)

	heights := hourBarHeights(hourly, plot.Height())
	rects := hourBarRects(plot, a.scale)
	idleH := px(baseOvBarIdleH, a.scale)

	for i := 0; i < 24; i++ {
		bar := rects[i]
		if bar.Empty() {
			continue
		}
		h := heights[i]
		if h <= 0 {
			// 这一小时没有按键：留 1–2px 浅灰占位，让「没有数据」本身可见。
			cv.fill(Rect{bar.Left, bar.Bottom - idleH, bar.Right, bar.Bottom}, colOvBarIdle)
			continue
		}
		cv.fill(Rect{bar.Left, bar.Bottom - h, bar.Right, bar.Bottom}, colAccent)
	}

	// 刻度标在对应柱子的正下方，0/3/6/…/21 共 8 个。
	if ticks.Empty() {
		return
	}
	tickW := px(baseOvTickW, a.scale)
	for k, label := range hourTickLabels() {
		idx := k * 3
		if idx >= len(rects) {
			break
		}
		bar := rects[idx]
		left := bar.Left + bar.Width()/2 - tickW/2
		if left < ticks.Left {
			left = ticks.Left
		}
		if right := left + tickW; right > ticks.Right {
			left = ticks.Right - tickW
		}
		cv.text(label, Rect{left, ticks.Top, left + tickW, ticks.Bottom},
			fontSmall, colTextOff, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
	}
}

// overviewText 用调用方自建的字体画一行字。
//
// paint.go 的 canvas.text 只认 fontID，而概览页的大号数字没有对应 fontID；
// 与其改 paint.go 的字体映射（会和同时在改界面的代码冲突），不如在本地
// 复制这十行 SelectObject/SetTextColor/DrawText 的配对。
func (cv canvas) overviewText(s string, r Rect, font syscall.Handle, color uint32, flags uint32) {
	if s == "" || r.Empty() {
		return
	}
	var old syscall.Handle
	if font != 0 {
		old = selectObject(cv.hdc, font)
	}
	setTextColor(cv.hdc, color)
	drawText(cv.hdc, s, r, flags)
	if old != 0 {
		selectObject(cv.hdc, old)
	}
}
