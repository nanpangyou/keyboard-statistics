package ui

import (
	"math"
	"testing"

	"keyboard-statistics/internal/stats"
)

// 键位表：5 行主键区，数量、行号、标准宽度单位都要对。
func TestComputeHeatmapLayoutRows(t *testing.T) {
	l := ComputeHeatmapLayout(Rect{Right: 900, Bottom: 560}, 1)
	if len(l.Keys) == 0 {
		t.Fatal("热力图没有键位")
	}

	wantRowCounts := []int{14, 14, 13, 11, 6}
	gotRowCounts := make([]int, len(wantRowCounts))
	for _, k := range l.Keys {
		if k.Row < 0 || k.Row >= len(gotRowCounts) {
			t.Fatalf("键 %q 的行号 %d 越界", k.Label, k.Row)
		}
		gotRowCounts[k.Row]++
	}
	for row, want := range wantRowCounts {
		if gotRowCounts[row] != want {
			t.Fatalf("第 %d 行键数 = %d，期望 %d", row, gotRowCounts[row], want)
		}
	}
}

// 左右 Ctrl / Shift / Alt 已经按口径合并，图上只能出现一个；
// Win 不合并，所以左右 Win 各保留一个。
func TestHeatmapLayoutMergesOnlyModifiers(t *testing.T) {
	l := ComputeHeatmapLayout(Rect{Right: 900, Bottom: 560}, 1)

	seen := make(map[uint16]int)
	for _, k := range l.Keys {
		seen[k.VK]++
	}
	for _, vk := range []uint16{stats.VKControl, stats.VKShift, stats.VKMenu} {
		if seen[vk] != 1 {
			t.Fatalf("%s 在热力图上出现 %d 次，期望 1 次", stats.KeyName(vk), seen[vk])
		}
	}
	if seen[stats.VKLControl] != 0 || seen[stats.VKRControl] != 0 ||
		seen[stats.VKLShift] != 0 || seen[stats.VKRShift] != 0 ||
		seen[stats.VKLMenu] != 0 || seen[stats.VKRMenu] != 0 {
		t.Fatalf("热力图上不应出现左右分开的修饰键：%v", seen)
	}
	if seen[stats.VKLWin] != 1 || seen[stats.VKRWin] != 1 {
		t.Fatalf("左右 Win 应各有一个，实际 %v", seen)
	}
}

// 所有键按标准宽度单位摆放：普通键 1u，Backspace 2u，Tab 1.5u，Shift 2.25u，空格 6.25u。
func TestHeatmapLayoutWidthUnits(t *testing.T) {
	l := ComputeHeatmapLayout(Rect{Right: 900, Bottom: 560}, 1)

	find := func(label string) HeatmapKey {
		t.Helper()
		for _, k := range l.Keys {
			if k.Label == label {
				return k
			}
		}
		t.Fatalf("找不到键 %q", label)
		return HeatmapKey{}
	}

	// 键盘整体宽度恒为 15u，用整张键盘反推 1u；相邻键的缝隙单独量。
	one := find("1")
	two := find("2")
	gap := two.Rect.Left - one.Rect.Right
	if gap <= 0 {
		t.Fatalf("相邻键缝隙 = %d，期望为正", gap)
	}
	unit := float64(l.Keyboard.Width()) / heatmapUnitsWide

	cases := []struct {
		label string
		units float64
	}{
		{"Backspace", 2},
		{"Tab", 1.5},
		{"Shift", 2.25},
		{"空格", 6.25},
	}

	for _, c := range cases {
		key := find(c.label)
		gotUnits := float64(key.Rect.Width()+gap) / unit
		if math.Abs(gotUnits-c.units) > 0.05 {
			t.Fatalf("%s 宽度 = %.3f 个基准单位，期望 %.2f", c.label, gotUnits, c.units)
		}
	}
}

// 布局必须整体落在内容区内，标题 / 提示 / 键盘 / 悬停行 / 脚注自上而下互不重叠。
func TestComputeHeatmapLayoutBounds(t *testing.T) {
	content := Rect{Left: 0, Top: 96, Right: 900, Bottom: 656}
	l := ComputeHeatmapLayout(content, 1)

	for name, r := range map[string]Rect{
		"标题":  l.Title,
		"提示":  l.Hint,
		"键盘":  l.Keyboard,
		"悬停行": l.Hover,
		"脚注":  l.Note,
	} {
		if r.Empty() {
			t.Fatalf("%s 区域为空：%+v", name, r)
		}
		if r.Left < content.Left || r.Right > content.Right ||
			r.Top < content.Top || r.Bottom > content.Bottom {
			t.Fatalf("%s 越出内容区：%+v content=%+v", name, r, content)
		}
	}

	if !(l.Title.Bottom <= l.Hint.Top) {
		t.Fatalf("提示应在标题下方：title=%+v hint=%+v", l.Title, l.Hint)
	}
	if !(l.Hint.Bottom <= l.Keyboard.Top) {
		t.Fatalf("键盘应在提示下方：hint=%+v keyboard=%+v", l.Hint, l.Keyboard)
	}
	if !(l.Keyboard.Bottom <= l.Hover.Top) {
		t.Fatalf("悬停行应在键盘下方：keyboard=%+v hover=%+v", l.Keyboard, l.Hover)
	}
	if !(l.Hover.Bottom <= l.Note.Top) {
		t.Fatalf("脚注应在悬停行下方：hover=%+v note=%+v", l.Hover, l.Note)
	}

	for _, k := range l.Keys {
		if k.Rect.Empty() {
			t.Fatalf("键 %q 尺寸为空：%+v", k.Label, k.Rect)
		}
		if k.Rect.Left < l.Keyboard.Left || k.Rect.Right > l.Keyboard.Right ||
			k.Rect.Top < l.Keyboard.Top || k.Rect.Bottom > l.Keyboard.Bottom {
			t.Fatalf("键 %q 越出键盘区域：%+v keyboard=%+v", k.Label, k.Rect, l.Keyboard)
		}
	}
}

// 同一行的键同高、行与行自上而下不重叠；键之间也不应互相重叠。
func TestHeatmapLayoutGeometry(t *testing.T) {
	l := ComputeHeatmapLayout(Rect{Right: 900, Bottom: 560}, 1)

	rowTop := make(map[int]int32)
	rowBottom := make(map[int]int32)
	for _, k := range l.Keys {
		if top, ok := rowTop[k.Row]; ok {
			if top != k.Rect.Top || rowBottom[k.Row] != k.Rect.Bottom {
				t.Fatalf("第 %d 行键高度不一致：%+v", k.Row, k.Rect)
			}
		} else {
			rowTop[k.Row] = k.Rect.Top
			rowBottom[k.Row] = k.Rect.Bottom
		}
	}

	for i := 0; i < len(l.Keys); i++ {
		for j := i + 1; j < len(l.Keys); j++ {
			if rectOverlap(l.Keys[i].Rect, l.Keys[j].Rect) {
				t.Fatalf("键 %q 与 %q 重叠：%+v %+v", l.Keys[i].Label, l.Keys[j].Label,
					l.Keys[i].Rect, l.Keys[j].Rect)
			}
		}
	}
}

// 内容区不够时整张键盘等比缩小，而不是把行拉扁或压出边界。
func TestComputeHeatmapLayoutScales(t *testing.T) {
	base := ComputeHeatmapLayout(Rect{Right: 900, Bottom: 560}, 1)
	double := ComputeHeatmapLayout(Rect{Right: 1800, Bottom: 1120}, 2)

	baseKey := findHeatmapKey(t, base, stats.VKA)
	doubleKey := findHeatmapKey(t, double, stats.VKA)
	if math.Abs(float64(doubleKey.Rect.Width())-2*float64(baseKey.Rect.Width())) > 3 {
		t.Fatalf("2 倍内容区下键宽 = %d，期望约为 %d", doubleKey.Rect.Width(), 2*baseKey.Rect.Width())
	}

	small := ComputeHeatmapLayout(Rect{Right: 420, Bottom: 320}, 1)
	smallKey := findHeatmapKey(t, small, stats.VKA)
	if smallKey.Rect.Width() <= 0 || smallKey.Rect.Height() <= 0 {
		t.Fatalf("窄内容区键尺寸非法：%+v", smallKey.Rect)
	}
	if small.Keyboard.Width() > 420 || small.Keyboard.Height() > 320 {
		t.Fatalf("窄内容区键盘越界：%+v", small.Keyboard)
	}
	ratio := float64(small.Keyboard.Width()) / float64(small.Keyboard.Height())
	if math.Abs(ratio-3) > 0.02 {
		t.Fatalf("键盘应按 15:5 等比缩放，得到 %+v（宽高比 %.3f）", small.Keyboard, ratio)
	}
}

// 着色只用 sqrt(count/max)，count == 0 或 max == 0 一律走中性灰，不能除零。
func TestHeatLevel(t *testing.T) {
	cases := []struct {
		name       string
		count, max int64
		want       float64
	}{
		{"最大值为 0", 100, 0, 0},
		{"次数为 0", 0, 100, 0},
		{"次数为负", -1, 100, 0},
		{"最大值", 100, 100, 1},
		{"四分之一", 25, 100, 0.5},
		{"十六分之一", 6, 96, 0.25},
		{"超过最大值", 120, 100, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := heatLevel(c.count, c.max)
			if math.Abs(got-c.want) > 1e-9 {
				t.Fatalf("heatLevel(%d,%d) = %v，期望 %v", c.count, c.max, got, c.want)
			}
		})
	}
}

// 颜色必须逐通道从浅灰线性插值到主题蓝 #2F6FEB。
func TestHeatColor(t *testing.T) {
	if got := heatColor(0, 100); got != heatEmptyColor {
		t.Fatalf("无数据颜色 = %#x，期望中性灰 %#x", got, heatEmptyColor)
	}
	if got := heatColor(100, 0); got != heatEmptyColor {
		t.Fatalf("max=0 颜色 = %#x，期望中性灰 %#x", got, heatEmptyColor)
	}
	if got := heatColor(100, 100); got != heatHighColor {
		t.Fatalf("最大键颜色 = %#x，期望主题蓝 %#x", got, heatHighColor)
	}

	if got := heatColorAt(0.5); got != heatRGB(0x8E, 0xB0, 0xF1) {
		t.Fatalf("50%% 插值颜色 = %#x，期望 %#x", got, heatRGB(0x8E, 0xB0, 0xF1))
	}
	if got := heatColorAt(-1); got != heatLowColor {
		t.Fatalf("t<0 应夹到浅灰，得到 %#x", got)
	}
	if got := heatColorAt(2); got != heatHighColor {
		t.Fatalf("t>1 应夹到主题蓝，得到 %#x", got)
	}
}

// 深色键上的文字必须自动换白，浅色键仍用深色，保证对比度。
func TestHeatTextColor(t *testing.T) {
	if got := heatTextColor(0, 100); got != heatTextOnLight {
		t.Fatalf("无数据键文字颜色 = %#x，期望深色 %#x", got, heatTextOnLight)
	}
	if got := heatTextColor(25, 100); got != heatTextOnLight {
		t.Fatalf("中低频键文字颜色 = %#x，期望深色 %#x", got, heatTextOnLight)
	}
	if got := heatTextColor(100, 100); got != heatTextOnDark {
		t.Fatalf("最高频键文字颜色 = %#x，期望白色 %#x", got, heatTextOnDark)
	}
}

// 命中测试沿用 Rect.Contains（右下开区间），缝隙与键盘外都不命中。
func TestHeatmapHitKey(t *testing.T) {
	l := ComputeHeatmapLayout(Rect{Right: 900, Bottom: 560}, 1)

	for _, want := range l.Keys {
		cx := (want.Rect.Left + want.Rect.Right) / 2
		cy := (want.Rect.Top + want.Rect.Bottom) / 2
		idx, ok := l.HitKey(cx, cy)
		if !ok {
			t.Fatalf("键 %q 中心 (%d,%d) 未命中", want.Label, cx, cy)
		}
		if l.Keys[idx].VK != want.VK || l.Keys[idx].Label != want.Label {
			t.Fatalf("键 %q 中心命中到 %q", want.Label, l.Keys[idx].Label)
		}
	}

	first := l.Keys[0]
	if _, ok := l.HitKey(first.Rect.Left, first.Rect.Top); !ok {
		t.Fatal("左上边界应命中（闭区间）")
	}
	if _, ok := l.HitKey(first.Rect.Right, first.Rect.Bottom); ok {
		t.Fatal("右下边界不应命中（开区间）")
	}
	if _, ok := l.HitKey(l.Keyboard.Left-1, l.Keyboard.Top-1); ok {
		t.Fatal("键盘外不应命中")
	}
	if _, ok := l.HitKey(l.Keyboard.Right, l.Keyboard.Bottom); ok {
		t.Fatal("键盘右下边界不应命中")
	}
}

// 悬停文案：键名取 stats.KeyName，占比取 stats.Percent（四舍五入，分母 0 时 0%）。
func TestHeatmapHoverText(t *testing.T) {
	cases := []struct {
		name  string
		vk    uint16
		count int64
		total int64
		want  string
	}{
		{"普通键", stats.VKA, 1234, 10000, "A · 1234 次 · 占全部按键 12%"},
		{"空数据", stats.VKSpace, 0, 0, "空格 · 0 次 · 占全部按键 0%"},
		{"四舍五入", stats.VK1, 1, 8, "1 · 1 次 · 占全部按键 13%"},
		{"中文键名", stats.VKControl, 55, 1000, "Ctrl · 55 次 · 占全部按键 6%"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := heatmapHoverText(c.vk, c.count, c.total); got != c.want {
				t.Fatalf("heatmapHoverText(%d,%d,%d) = %q，期望 %q",
					c.vk, c.count, c.total, got, c.want)
			}
		})
	}
}

// 旧库可能残留左右修饰键的桶；热力图按通用码展示，取数时也要合并，
// 否则升级后 Shift 仍然只算新数据、看起来像丢了一半。
func TestHeatmapKeyCountLegacyModifiers(t *testing.T) {
	counts := map[uint16]int64{
		stats.VKControl: 2, stats.VKLControl: 3, stats.VKRControl: 4,
		stats.VKShift: 0, stats.VKLShift: 5, stats.VKRShift: 6,
		stats.VKMenu: 7, stats.VKLMenu: 0, stats.VKRMenu: 2,
		stats.VKA:    4,
		stats.VKLWin: 8, stats.VKRWin: 9,
	}
	cases := []struct {
		vk   uint16
		want int64
	}{
		{stats.VKControl, 9},
		{stats.VKShift, 11},
		{stats.VKMenu, 9},
		{stats.VKA, 4},
		{stats.VKLWin, 8},
		{stats.VKRWin, 9},
	}
	for _, c := range cases {
		if got := heatmapKeyCount(counts, c.vk); got != c.want {
			t.Fatalf("heatmapKeyCount(0x%X) = %d，期望 %d", c.vk, got, c.want)
		}
	}
	if got := heatmapKeyCount(nil, stats.VKShift); got != 0 {
		t.Fatalf("空数据 heatmapKeyCount = %d，期望 0", got)
	}
}

// 归一化只取「图内」的最大值：没画出来的键（如 F5）不能把整张键盘压成浅色。
func TestHeatmapMaxCount(t *testing.T) {
	l := ComputeHeatmapLayout(Rect{Right: 900, Bottom: 560}, 1)
	counts := map[uint16]int64{
		stats.VKA: 120,
		stats.VK1: 80,
		0x70:      9999, // F5，不在图上
	}
	if got := heatmapMaxCount(l.Keys, counts); got != 120 {
		t.Fatalf("图内最大次数 = %d，期望 120", got)
	}
	if got := heatmapMaxCount(l.Keys, nil); got != 0 {
		t.Fatalf("空数据最大次数 = %d，期望 0", got)
	}
}

func findHeatmapKey(t *testing.T, l HeatmapLayout, vk uint16) HeatmapKey {
	t.Helper()
	for _, k := range l.Keys {
		if k.VK == vk {
			return k
		}
	}
	t.Fatalf("找不到 vk=0x%X 的键", vk)
	return HeatmapKey{}
}
