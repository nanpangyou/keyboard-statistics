//go:build windows

package ui

import "keyboard-statistics/internal/config"

// paintSettings 画设置页。整页是自绘的，所有可点区域都来自 ComputeSettingsLayout，
// 所以「画在哪」和「点到哪」永远一致。
func (a *App) paintSettings(cv canvas, content Rect) {
	s := ComputeSettingsLayout(content, a.scale)
	hovered := func(hit SettingsHit) bool {
		return a.hover.kind == hoverSettings && a.hover.index == int(hit)
	}

	// —— 采集 ——
	cv.text("采集", s.SectionCollect, fontSection, colText, dtVCenter|dtSingleLine|dtNoPrefix)
	collectStyle := btnSecondary
	if !a.collecting {
		// 暂停时把「继续采集」做成主按钮，用户一眼知道该点哪里恢复。
		collectStyle = btnPrimary
	}
	cv.button(s.CollectBtn, collectButtonText(a.collecting), collectStyle, hovered(SettingsHitCollect))
	cv.text("暂停后敲键盘不会被计入统计。管理员权限窗口（如任务管理器）里的按键收不到，这是 Windows 的限制。",
		s.CollectHint, fontSmall, colTextDim, dtVCenter|dtSingleLine|dtNoPrefix|dtEndEllipsis)

	// —— 开机自启 ——
	cv.text("开机自启", s.SectionAutostart, fontSection, colText, dtVCenter|dtSingleLine|dtNoPrefix)
	autostartLabel := "开机时自动启动（启动后最小化到任务栏）"
	if !a.settings.autostartAvailable {
		autostartLabel = "开机时自动启动（当前环境不可用）"
	}
	cv.checkbox(s.AutostartBox, autostartLabel,
		a.settings.autostartChecked, a.settings.autostartAvailable, hovered(SettingsHitAutostart))
	if warn := staleWarningText(a.settings.autostartStale); warn != "" {
		cv.text(warn, s.StaleHint, fontSmall, colWarn, dtVCenter|dtSingleLine|dtNoPrefix|dtEndEllipsis)
	}

	// —— 数据 ——
	cv.text("数据", s.SectionData, fontSection, colText, dtVCenter|dtSingleLine|dtNoPrefix)
	cv.button(s.ExportBtn, "导出 CSV…", btnSecondary, hovered(SettingsHitExport))
	cv.button(s.ClearBtn, "清空全部数据…", btnDanger, hovered(SettingsHitClear))
	cv.text("导出两个 UTF-8（带 BOM）的 CSV：按键明细与组合键明细，Excel 可直接打开。",
		s.DataHint, fontSmall, colTextDim, dtVCenter|dtSingleLine|dtNoPrefix|dtEndEllipsis)

	// —— 关于 ——
	cv.text("关于", s.SectionAbout, fontSection, colText, dtVCenter|dtSingleLine|dtNoPrefix)
	cv.button(s.OpenDirBtn, "打开数据目录", btnSecondary, hovered(SettingsHitOpenDataDir))

	dir, err := config.Dir()
	dirText := dir
	if err != nil {
		dirText = "（无法定位数据目录）"
	}
	pad := px(baseSetPad, a.scale)
	for i, line := range aboutLines(a.opt.Version, dirText) {
		r := Rect{
			s.Content.Left + pad,
			s.AboutTop + int32(i)*s.LineH,
			s.Content.Right - pad,
			s.AboutTop + int32(i+1)*s.LineH,
		}
		cv.text(line, r, fontBody, colTextDim, dtVCenter|dtSingleLine|dtNoPrefix|dtEndEllipsis)
	}
}
