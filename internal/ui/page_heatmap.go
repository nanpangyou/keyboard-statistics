package ui

import (
	"fmt"
	"math"

	"keyboard-statistics/internal/stats"
)

// 热力图的键位表与着色规则是纯计算，放在无 build tag 的文件里，
// 这样非 Windows 平台也能跑同一套表驱动测试；真正 GDI 绘制在同名 _windows 文件里。

const (
	// 主键区按 US QWERTY 的 15u × 5u 网格摆放。u 是普通键宽，宽键用多个 u。
	heatmapUnitsWide = 15.0
	heatmapUnitsHigh = 5.0

	heatmapPadX        = 28
	heatmapPadY        = 14
	heatmapTitleH      = 26
	heatmapHintH       = 20
	heatmapHoverH      = 24
	heatmapNoteH       = 20
	heatmapTitleGap    = 2
	heatmapHoverGap    = 6
	heatmapKeyboardGap = 10

	// 浅灰 → 主题蓝的插值阈值：超过这个 t 用白字，保证深色键上的可读性。
	heatTextWhiteLevel = 0.72
)

// 热力图页固定文案。和布局分开，避免测试里反复写死中文。
const (
	heatmapTitleText  = "按键热力图"
	heatmapHintText   = "颜色越深，按得越多"
	heatmapHoverIdle  = "悬停到键位上查看次数与占比"
	heatmapEmptyText  = "当前时间范围还没有按键记录，敲几下键盘试试"
	heatmapMergedNote = "左右 Ctrl / Shift / Alt 已合并统计"
)

// 热力图专用颜色。paint.go 的调色板带 Windows build tag，纯函数层不能依赖它，
// 所以这里用同一套 COLORREF 规则单独定义；高色与主题强调色保持一致。
var (
	heatEmptyColor   = heatRGB(0xEC, 0xEC, 0xEC)
	heatLowColor     = heatRGB(0xED, 0xF1, 0xF7)
	heatHighColor    = heatRGB(0x2F, 0x6F, 0xEB)
	heatTextOnLight  = heatRGB(0x1F, 0x1F, 0x1F)
	heatTextOnDark   = heatRGB(0xFF, 0xFF, 0xFF)
	heatHoverLine    = heatRGB(0x25, 0x5D, 0xD0)
	heatKeyEmptyLine = heatRGB(0xD9, 0xDC, 0xE1)
)

// HeatmapKey 是键位图上的一个键：键帽文字、统计键码、所在行与像素矩形。
type HeatmapKey struct {
	Label string
	VK    uint16
	Row   int
	Rect  Rect
}

// HeatmapLayout 是热力图页的像素布局。
//
// 键盘本体始终按 15:5 等比缩放，不用窗口宽高各自拉伸——否则窄窗口里的
// Backspace / 空格等宽键会变形，用户会误以为键位表和真实键盘对不上。
type HeatmapLayout struct {
	Content  Rect
	Title    Rect
	Hint     Rect
	Keyboard Rect
	Hover    Rect
	Note     Rect

	Keys []HeatmapKey
}

// heatmapCell 是键位表的单元格。空白单元格用来保留被合并的右 Ctrl / 右 Alt /
// 右 Shift 的位置，让空格仍居中、其他行继续保持标准键位。
type heatmapCell struct {
	label string
	vk    uint16
	width float64
	blank bool
}

func heatKey(label string, vk uint16) heatmapCell {
	return heatmapCell{label: label, vk: vk, width: 1}
}

func heatWideKey(label string, vk uint16, width float64) heatmapCell {
	return heatmapCell{label: label, vk: vk, width: width}
}

func heatSpacer(width float64) heatmapCell {
	return heatmapCell{width: width, blank: true}
}

// heatmapRows 是 5 行主键区。宽键按标准宽度单位：
// 1u 普通键、2u Backspace、1.5u Tab / 反斜杠、1.75u CapsLock、2.25u Enter / 左 Shift、
// 2.75u 空出来的右 Shift、6.25u 空格、1.25u 底部修饰键。
//
// 左右 Ctrl / Shift / Alt 在采集入口已合并，所以这里只保留左侧一个；右侧对应位置
// 用 spacer 占位而不是画第二个键。Win 不参与合并，左右 Win 都保留。
func heatmapRows() [][]heatmapCell {
	return [][]heatmapCell{
		{
			heatKey("`", stats.VKOEM3),
			heatKey("1", 0x31),
			heatKey("2", 0x32),
			heatKey("3", 0x33),
			heatKey("4", 0x34),
			heatKey("5", 0x35),
			heatKey("6", 0x36),
			heatKey("7", 0x37),
			heatKey("8", 0x38),
			heatKey("9", 0x39),
			heatKey("0", 0x30),
			heatKey("-", stats.VKOEMMinus),
			heatKey("=", stats.VKOEMPlus),
			heatWideKey("Backspace", stats.VKBack, 2),
		},
		{
			heatWideKey("Tab", stats.VKTab, 1.5),
			heatKey("Q", 0x51),
			heatKey("W", 0x57),
			heatKey("E", 0x45),
			heatKey("R", 0x52),
			heatKey("T", 0x54),
			heatKey("Y", 0x59),
			heatKey("U", 0x55),
			heatKey("I", 0x49),
			heatKey("O", 0x4F),
			heatKey("P", 0x50),
			heatKey("[", stats.VKOEM4),
			heatKey("]", stats.VKOEM6),
			heatWideKey("\\", stats.VKOEM5, 1.5),
		},
		{
			heatWideKey("Caps", stats.VKCaps, 1.75),
			heatKey("A", 0x41),
			heatKey("S", 0x53),
			heatKey("D", 0x44),
			heatKey("F", 0x46),
			heatKey("G", 0x47),
			heatKey("H", 0x48),
			heatKey("J", 0x4A),
			heatKey("K", 0x4B),
			heatKey("L", 0x4C),
			heatKey(";", stats.VKOEM1),
			heatKey("'", stats.VKOEM7),
			heatWideKey("Enter", stats.VKEnter, 2.25),
		},
		{
			heatWideKey("Shift", stats.VKShift, 2.25),
			heatKey("Z", 0x5A),
			heatKey("X", 0x58),
			heatKey("C", 0x43),
			heatKey("V", 0x56),
			heatKey("B", 0x42),
			heatKey("N", 0x4E),
			heatKey("M", 0x4D),
			heatKey(",", stats.VKOEMComma),
			heatKey(".", stats.VKOEMDot),
			heatKey("/", stats.VKOEM2),
			heatSpacer(2.75), // 右 Shift 已合并，不重复画
		},
		{
			heatWideKey("Ctrl", stats.VKControl, 1.25),
			heatWideKey("Win", stats.VKLWin, 1.25),
			heatWideKey("Alt", stats.VKMenu, 1.25),
			heatWideKey("空格", stats.VKSpace, 6.25),
			heatSpacer(1.25), // 右 Alt 已合并
			heatWideKey("菜单", stats.VKApps, 1.25),
			heatWideKey("Win", stats.VKRWin, 1.25),
			heatSpacer(1.25), // 右 Ctrl 已合并
		},
	}
}

// ComputeHeatmapLayout 计算热力图页布局，并把 5 行键位表换算成像素矩形。
//
// 键盘区域按可用空间整体等比缩放并居中；内容区小到放不下时返回空 Keys，
// 绘制层按空布局处理，不会画出面积为负的矩形。
func ComputeHeatmapLayout(content Rect, scale float64) HeatmapLayout {
	if scale <= 0 {
		scale = 1
	}

	padX := px(heatmapPadX, scale)
	padY := px(heatmapPadY, scale)
	x1 := content.Left + padX
	x2 := content.Right - padX
	if x2 <= x1 {
		return HeatmapLayout{Content: content}
	}

	y := content.Top + padY
	l := HeatmapLayout{Content: content}

	l.Title = Rect{x1, y, x2, y + px(heatmapTitleH, scale)}
	y = l.Title.Bottom + px(heatmapTitleGap, scale)

	l.Hint = Rect{x1, y, x2, y + px(heatmapHintH, scale)}

	noteH := px(heatmapNoteH, scale)
	hoverH := px(heatmapHoverH, scale)
	l.Note = Rect{x1, content.Bottom - padY - noteH, x2, content.Bottom - padY}
	l.Hover = Rect{x1, l.Note.Top - px(heatmapHoverGap, scale) - hoverH, x2, l.Note.Top - px(heatmapHoverGap, scale)}

	region := Rect{
		Left:   x1,
		Top:    l.Hint.Bottom + px(heatmapKeyboardGap, scale),
		Right:  x2,
		Bottom: l.Hover.Top - px(heatmapKeyboardGap, scale),
	}
	if region.Empty() {
		return l
	}

	unit := math.Min(float64(region.Width())/heatmapUnitsWide, float64(region.Height())/heatmapUnitsHigh)
	if unit < 1 {
		return l
	}

	kbW := int32(math.Round(heatmapUnitsWide * unit))
	kbH := int32(math.Round(heatmapUnitsHigh * unit))
	l.Keyboard = Rect{
		Left:   region.Left + (region.Width()-kbW)/2,
		Top:    region.Top + (region.Height()-kbH)/2,
		Right:  region.Left + (region.Width()-kbW)/2 + kbW,
		Bottom: region.Top + (region.Height()-kbH)/2 + kbH,
	}

	gap := int32(math.Round(unit * 0.06))
	if gap < 1 {
		gap = 1
	}

	for row, cells := range heatmapRows() {
		var x float64
		for _, cell := range cells {
			if !cell.blank {
				left := l.Keyboard.Left + int32(math.Round(x*unit))
				right := l.Keyboard.Left + int32(math.Round((x+cell.width)*unit)) - gap
				top := l.Keyboard.Top + int32(math.Round(float64(row)*unit))
				bottom := l.Keyboard.Top + int32(math.Round(float64(row+1)*unit)) - gap
				if right > left && bottom > top {
					l.Keys = append(l.Keys, HeatmapKey{
						Label: cell.label,
						VK:    cell.vk,
						Row:   row,
						Rect:  Rect{left, top, right, bottom},
					})
				}
			}
			x += cell.width
		}
	}

	return l
}

// HitKey 返回命中的键下标，语义与 Rect.Contains 一致：左闭右开，
// 相邻键在右/下边界上不会重复命中。
func (l HeatmapLayout) HitKey(x, y int32) (int, bool) {
	for i, k := range l.Keys {
		if k.Rect.Contains(x, y) {
			return i, true
		}
	}
	return 0, false
}

// heatLevel 把次数压到 0..1：sqrt 曲线放大低中频段，避免一两个高频键
// 把整张热力图压成同一档颜色。
func heatLevel(count, max int64) float64 {
	if count <= 0 || max <= 0 {
		return 0
	}
	if count >= max {
		return 1
	}
	return math.Sqrt(float64(count) / float64(max))
}

// heatColorAt 在浅灰与主题蓝之间逐通道线性插值，t 自动夹到 0..1。
func heatColorAt(t float64) uint32 {
	if t <= 0 {
		return heatLowColor
	}
	if t >= 1 {
		return heatHighColor
	}
	lerp := func(lo, hi byte) byte {
		return byte(math.Round(float64(lo) + (float64(hi)-float64(lo))*t))
	}
	return heatRGB(
		lerp(0xED, 0x2F),
		lerp(0xF1, 0x6F),
		lerp(0xF7, 0xEB),
	)
}

// heatColor 返回按键的填充色。没有数据（count==0）或整图无数据（max==0）
// 都走中性灰，绝不参与渐变，避免除零。
func heatColor(count, max int64) uint32 {
	if count <= 0 || max <= 0 {
		return heatEmptyColor
	}
	return heatColorAt(heatLevel(count, max))
}

// heatTextColor 在深色填充上自动改用白字；浅色填充继续用深色字。
func heatTextColor(count, max int64) uint32 {
	if count <= 0 || max <= 0 {
		return heatTextOnLight
	}
	if heatLevel(count, max) >= heatTextWhiteLevel {
		return heatTextOnDark
	}
	return heatTextOnLight
}

// heatKeyBorderColor 是普通键的 1px 边框：无数据用浅灰，有数据用主题蓝，
// 让浅色档位也能看出一格一格的键位。
func heatKeyBorderColor(count, max int64) uint32 {
	if count <= 0 || max <= 0 {
		return heatKeyEmptyLine
	}
	return heatHighColor
}

// heatHoverBorderColor 是悬停键的 2px 描边。深色键用白色描边，
// 浅色键用更深的蓝色，两种情况都能和填充色拉开对比。
func heatHoverBorderColor(count, max int64) uint32 {
	if count > 0 && max > 0 && heatLevel(count, max) >= heatTextWhiteLevel {
		return heatTextOnDark
	}
	return heatHoverLine
}

// heatmapKeyCount 取某个键的展示次数。
//
// 新数据在采集入口已经过 NormalizeVK，但旧库里可能还留着 VK_LSHIFT / VK_RSHIFT
// 这类桶；热力图上只画一个通用键，所以这里把旧桶一并加进来，否则升级后
// Shift 会看起来只统计了升级后的部分。
func heatmapKeyCount(counts map[uint16]int64, vk uint16) int64 {
	var count int64
	for raw, c := range counts {
		if stats.NormalizeVK(raw) == vk {
			count += c
		}
	}
	return count
}

// heatmapMaxCount 只取「图上出现过的键」的最大值。
// Snapshot.KeyCounts 里可能还有功能键、小键盘等没画出来的键，不能拿它们做归一化。
func heatmapMaxCount(keys []HeatmapKey, counts map[uint16]int64) int64 {
	var max int64
	for _, k := range keys {
		if c := heatmapKeyCount(counts, k.VK); c > max {
			max = c
		}
	}
	return max
}

// heatmapHoverText 拼图下方的一行悬停详情。占比按 metrics.md 第 4 节，
// 分母是范围内全部单键次数；分母为 0 时 stats.Percent 返回 0。
func heatmapHoverText(vk uint16, count, total int64) string {
	if count < 0 {
		count = 0
	}
	return fmt.Sprintf("%s · %d 次 · 占全部按键 %d%%", stats.KeyName(vk), count, stats.Percent(count, total))
}

// heatRGB 与 paint.go 的 rgb() 使用同一套 COLORREF 字节序（0x00BBGGRR）。
func heatRGB(r, g, b byte) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}
