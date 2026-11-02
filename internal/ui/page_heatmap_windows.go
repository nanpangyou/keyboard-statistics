//go:build windows

package ui

import "fmt"

// paintHeatmap 画 5 行 QWERTY 主键区：按当前范围的按键次数做 sqrt 热力着色，
// 悬停键额外描边，并在图下方显示键名、次数与占全部单键的百分比。
func (a *App) paintHeatmap(cv canvas, content Rect) {
	l := ComputeHeatmapLayout(content, a.scale)
	counts := a.snap.KeyCounts
	total := a.snap.TotalKeys
	maxCount := heatmapMaxCount(l.Keys, counts)

	cv.text(heatmapTitleText, l.Title, fontSection, colText,
		dtVCenter|dtSingleLine|dtNoPrefix)

	summary := fmt.Sprintf("总按键 %d 次", total)
	if total == 0 {
		summary = "暂无按键记录"
	}
	cv.text(summary, l.Title, fontSmall, colTextDim,
		dtRight|dtVCenter|dtSingleLine|dtNoPrefix|dtEndEllipsis)

	cv.text(heatmapHintText, l.Hint, fontSmall, colTextDim,
		dtVCenter|dtSingleLine|dtNoPrefix)

	for i, k := range l.Keys {
		count := heatmapKeyCount(counts, k.VK)
		cv.fill(k.Rect, heatColor(count, maxCount))

		if a.hover.kind == hoverHeatmap && a.hover.index == i {
			border := heatHoverBorderColor(count, maxCount)
			// 画两层 1px 矩形，得到 2px 悬停描边；不额外引 GDI 画笔。
			cv.border(k.Rect, border)
			cv.border(Rect{k.Rect.Left + 1, k.Rect.Top + 1, k.Rect.Right - 1, k.Rect.Bottom - 1}, border)
		} else {
			cv.border(k.Rect, heatKeyBorderColor(count, maxCount))
		}

		label := Rect{
			k.Rect.Left + px(2, a.scale),
			k.Rect.Top,
			k.Rect.Right - px(2, a.scale),
			k.Rect.Bottom,
		}
		cv.text(k.Label, label, fontSmall, heatTextColor(count, maxCount),
			dtCenter|dtVCenter|dtSingleLine|dtNoPrefix|dtEndEllipsis)
	}

	hoverText, hoverColor := heatmapHoverIdle, colTextOff
	if total == 0 {
		hoverText, hoverColor = heatmapEmptyText, colTextDim
	}
	if a.hover.kind == hoverHeatmap && a.hover.index >= 0 && a.hover.index < len(l.Keys) {
		k := l.Keys[a.hover.index]
		hoverText = heatmapHoverText(k.VK, heatmapKeyCount(counts, k.VK), total)
		hoverColor = colText
	}
	cv.text(hoverText, l.Hover, fontBody, hoverColor,
		dtCenter|dtVCenter|dtSingleLine|dtNoPrefix|dtEndEllipsis)

	cv.text(heatmapMergedNote, l.Note, fontSmall, colTextOff,
		dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
}
