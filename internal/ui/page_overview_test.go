package ui

import (
	"strings"
	"testing"

	"keyboard-statistics/internal/stats"
)

// 万/亿格式化是概览页指标卡的第一道门面：口径表（docs/metrics.md §9）要求千分位，
// 而总按键数很容易到万、亿量级，卡片宽度只有两百来像素，直接堆数字会被截断。
// 这里把边界（9999/10000、99999999/100000000）钉死，避免以后改成 float 四舍五入跑偏。
func TestFormatKeyCount(t *testing.T) {
	cases := []struct {
		name string
		in   int64
		want string
	}{
		{"零", 0, "0"},
		{"小数字原样", 42, "42"},
		{"三位不加分隔", 999, "999"},
		{"四位数千分位", 1000, "1,000"},
		{"四位数千分位 2", 1234, "1,234"},
		{"不足一万仍用千分位", 9999, "9,999"},
		{"整一万", 10000, "1 万"},
		{"一万二（一位小数）", 12345, "1.2 万"},
		{"一万零五百进位", 10500, "1.1 万"},
		{"九万九千九百九十九进位到十万", 99999, "10 万"},
		{"百万级", 1234567, "123.5 万"},
		{"整一亿", 100000000, "1 亿"},
		{"一亿二千三百四十五万（两位小数）", 123456789, "1.23 亿"},
		{"整两亿", 200000000, "2 亿"},
		{"负数按零处理", -5, "0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formatKeyCount(c.in); got != c.want {
				t.Fatalf("formatKeyCount(%d) = %q，期望 %q", c.in, got, c.want)
			}
		})
	}
}

// 活跃时长文案必须和 docs/metrics.md §8 完全一致：
// 「3 小时 12 分」/ 不足 1 小时只写「12 分」/ 不足 1 分钟写「不足 1 分」。
func TestFormatActiveDuration(t *testing.T) {
	cases := []struct {
		name string
		in   int64
		want string
	}{
		{"零秒", 0, "不足 1 分"},
		{"59 秒", 59, "不足 1 分"},
		{"整一分钟", 60, "1 分"},
		{"两分零五秒向下取整", 125, "2 分"},
		{"差一秒到一小时仍是分", 3599, "59 分"},
		{"整一小时", 3600, "1 小时"},
		{"一小时十二分", 4320, "1 小时 12 分"},
		{"二十二小时十二分", 79920, "22 小时 12 分"},
		{"负数按零处理", -1, "不足 1 分"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formatActiveDuration(c.in); got != c.want {
				t.Fatalf("formatActiveDuration(%d) = %q，期望 %q", c.in, got, c.want)
			}
		})
	}
}

// 峰值卡副标题按 metrics.md §10 只显示「月-日」，无效日期不能把半截字符串漏到界面上。
func TestFormatMonthDay(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"标准日期", "2026-11-02", "11-02"},
		{"空串", "", ""},
		{"脏数据", "garbage", ""},
		{"缺位", "2026-1-2", ""},
		{"带时间后缀", "2026-11-02 10:00", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formatMonthDay(c.in); got != c.want {
				t.Fatalf("formatMonthDay(%q) = %q，期望 %q", c.in, got, c.want)
			}
		})
	}
}

// 四张指标卡的标题、数值、副标题是用户第一眼看到的东西，逐字段断言。
func TestOverviewCardSpecs(t *testing.T) {
	m := overviewMetrics{
		TotalKeys:     12345,
		ActiveSeconds: 4320,
		DailyAverage:  1763,
		PeakCount:     2345,
		PeakDay:       "2026-11-02",
		Days:          7,
	}
	cards := overviewCardSpecs(m)
	if len(cards) != 4 {
		t.Fatalf("指标卡数量 = %d，期望 4", len(cards))
	}

	want := [4]cardSpec{
		{Title: "总按键数", Value: "1.2 万", Sub: "不含组合键"},
		{Title: "活跃时长", Value: "1 小时 12 分", Sub: "间隔 60 秒算同一会话"},
		{Title: "日均按键", Value: "1,763", Sub: "共 7 天"},
		{Title: "单日峰值", Value: "2,345", Sub: "11-02"},
	}
	for i := range want {
		if cards[i] != want[i] {
			t.Fatalf("第 %d 张卡 = %+v，期望 %+v", i+1, cards[i], want[i])
		}
	}
}

// 没有数据时数字一律显示「—」，不能显示 0：0 会被用户当成「统计到了 0 次」。
// 天数未知时副标题留空，不要写「共 0 天」。
func TestOverviewCardSpecsEmpty(t *testing.T) {
	cards := overviewCardSpecs(overviewMetrics{})
	for i, c := range cards {
		if c.Value != "—" {
			t.Fatalf("第 %d 张卡无数据时数值 = %q，期望「—」", i+1, c.Value)
		}
		if c.Title == "" {
			t.Fatalf("第 %d 张卡标题为空", i+1)
		}
	}
	if cards[2].Sub != "" {
		t.Fatalf("天数为 0 时日均卡副标题 = %q，期望空串", cards[2].Sub)
	}
	if cards[3].Sub != "" {
		t.Fatalf("无峰值日期时峰值卡副标题 = %q，期望空串", cards[3].Sub)
	}
}

// TOP 列表一行四个字段：序号 / 键名 / 次数 / 占比。键名必须走 stats.KeyName，
// 否则同一颗键在概览、热力图、CSV 里可能显示成不同名字。
// Raw 是给行内比例条用的原始次数，必须与显示用的 Count 同源。
func TestOverviewKeyRows(t *testing.T) {
	keys := []stats.KeyCount{
		{VK: 0x41, Count: 60}, // A
		{VK: 0x20, Count: 40}, // 空格
	}
	rows := overviewKeyRows(keys, 100)
	if len(rows) != 2 {
		t.Fatalf("行数 = %d，期望 2", len(rows))
	}
	want := []overviewRow{
		{Rank: "1", Name: "A", Count: "60", Percent: "60%", Raw: 60},
		{Rank: "2", Name: "空格", Count: "40", Percent: "40%", Raw: 40},
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("第 %d 行 = %+v，期望 %+v", i+1, rows[i], want[i])
		}
	}
}

// 组合键占比的分母是「组合键总次数」，不是总按键数（metrics.md §4 明确要求两套分母独立）。
func TestOverviewComboRows(t *testing.T) {
	combos := []stats.ComboCount{
		{Combo: "Ctrl+C", Count: 3},
		{Combo: "Alt+Tab", Count: 1},
	}
	rows := overviewComboRows(combos, comboTotal(map[string]int64{"Ctrl+C": 3, "Alt+Tab": 1, "Ctrl+V": 6}))
	if len(rows) != 2 {
		t.Fatalf("行数 = %d，期望 2", len(rows))
	}
	// 分母是全部组合总数 10，而不是这两行的 4。
	if rows[0].Percent != "30%" || rows[1].Percent != "10%" {
		t.Fatalf("组合占比 = %q/%q，期望 30%%/10%%", rows[0].Percent, rows[1].Percent)
	}
	if rows[0].Name != "Ctrl+C" || rows[0].Count != "3" || rows[0].Rank != "1" {
		t.Fatalf("首行 = %+v，期望 {1 Ctrl+C 3 30%%}", rows[0])
	}
}

// 分母为 0 时按 metrics.md §4 显示 0%，不能崩也不能出现 NaN%。
func TestOverviewPercentZeroTotal(t *testing.T) {
	rows := overviewKeyRows([]stats.KeyCount{{VK: 0x41, Count: 5}}, 0)
	if len(rows) != 1 || rows[0].Percent != "0%" {
		t.Fatalf("分母为 0 的占比 = %+v，期望 0%%", rows)
	}
}

// 列表最多 10 行：数据层已经截断，但界面不能假设它一定截断过。
func TestOverviewRowsCap(t *testing.T) {
	keys := make([]stats.KeyCount, 0, 14)
	for i := 0; i < 14; i++ {
		keys = append(keys, stats.KeyCount{VK: uint16(0x41 + i), Count: int64(100 - i)})
	}
	if got := len(overviewKeyRows(keys, 1000)); got != 10 {
		t.Fatalf("按键行数 = %d，期望 10", got)
	}

	combos := make([]stats.ComboCount, 0, 14)
	for i := 0; i < 14; i++ {
		combos = append(combos, stats.ComboCount{Combo: "Ctrl+A", Count: int64(100 - i)})
	}
	if got := len(overviewComboRows(combos, 1000)); got != 10 {
		t.Fatalf("组合行数 = %d，期望 10", got)
	}
}

func TestComboTotal(t *testing.T) {
	if got := comboTotal(nil); got != 0 {
		t.Fatalf("空 map 总数 = %d，期望 0", got)
	}
	if got := comboTotal(map[string]int64{"Ctrl+C": 3, "Alt+Tab": 2}); got != 5 {
		t.Fatalf("总数 = %d，期望 5", got)
	}
	// 脏数据（负数）不能把分母拉小到 0 以下。
	if got := comboTotal(map[string]int64{"Ctrl+C": 3, "bad": -10}); got != 3 {
		t.Fatalf("含负数总数 = %d，期望 3", got)
	}
}

func TestHourlyMax(t *testing.T) {
	if got := hourlyMax([24]int64{}); got != 0 {
		t.Fatalf("空分布最大值 = %d，期望 0", got)
	}
	var h [24]int64
	h[3], h[17] = 7, 42
	if got := hourlyMax(h); got != 42 {
		t.Fatalf("最大值 = %d，期望 42", got)
	}
	h[5] = -9
	if got := hourlyMax(h); got != 42 {
		t.Fatalf("含负数时最大值 = %d，期望 42", got)
	}
}

// 柱高换算：max=0 必须整体返回 0（这正是除零的入口），非零最小值至少 1px 才能看见。
func TestHourBarHeights(t *testing.T) {
	cases := []struct {
		name    string
		hourly  [24]int64
		chartH  int32
		wantIdx [3]int
		want    [3]int32
	}{
		{"全零不画柱", [24]int64{}, 100, [3]int{0, 1, 2}, [3]int32{0, 0, 0}},
		{"高度为零", [24]int64{0: 10}, 0, [3]int{0, 1, 2}, [3]int32{0, 0, 0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := hourBarHeights(c.hourly, c.chartH)
			for i, idx := range c.wantIdx {
				if got[idx] != c.want[i] {
					t.Fatalf("hourBarHeights[%d] = %d，期望 %d", idx, got[idx], c.want[i])
				}
			}
		})
	}

	var h [24]int64
	h[0], h[1], h[2] = 50, 100, 1
	got := hourBarHeights(h, 100)
	if got[0] != 50 || got[1] != 100 || got[2] != 1 {
		t.Fatalf("按比例换算 = %d/%d/%d，期望 50/100/1", got[0], got[1], got[2])
	}

	// 极小的非零值也要留 1px，否则「有数据」看起来和「没数据」一样。
	h = [24]int64{}
	h[0], h[1] = 1, 1000000
	if got := hourBarHeights(h, 80); got[0] != 1 {
		t.Fatalf("最小值柱高 = %d，期望至少 1", got[0])
	}

	// 负数计数（脏数据）当作 0，不许画出负高度。
	h = [24]int64{}
	h[0] = -5
	if got := hourBarHeights(h, 80); got[0] != 0 {
		t.Fatalf("负数柱高 = %d，期望 0", got[0])
	}
}

// 24 根柱必须落在给定区域里、互不重叠，间隔 4–6px（docs/ui-plan.md §6.2）。
func TestHourBarRects(t *testing.T) {
	area := Rect{Left: 20, Top: 534, Right: 880, Bottom: 626}
	rects := hourBarRects(area, 1)
	if len(rects) != 24 {
		t.Fatalf("柱子数量 = %d，期望 24", len(rects))
	}
	if rects[0].Left != area.Left {
		t.Fatalf("首柱左侧 = %d，期望贴着区域左边 %d", rects[0].Left, area.Left)
	}
	if rects[23].Right > area.Right {
		t.Fatalf("末柱右侧 = %d，越出区域右边 %d", rects[23].Right, area.Right)
	}
	if area.Right-rects[23].Right >= 24 {
		t.Fatalf("右侧空白 %d px，柱宽分配过于保守", area.Right-rects[23].Right)
	}
	for i := 0; i < 24; i++ {
		r := rects[i]
		if r.Width() < 1 || r.Height() != area.Height() {
			t.Fatalf("第 %d 根柱 = %+v，期望宽度 >= 1 且高度等于 %d", i, r, area.Height())
		}
		if i > 0 {
			gap := r.Left - rects[i-1].Right
			if gap < 4 || gap > 6 {
				t.Fatalf("第 %d/%d 根柱间隔 = %d，期望 4–6px", i-1, i, gap)
			}
			if gap <= 0 {
				t.Fatalf("柱子重叠：%+v 与 %+v", rects[i-1], r)
			}
		}
	}
}

// 过窄的区域不能让柱子算出负宽度或越界——宽度不够时宁可缩小间隔。
func TestHourBarRectsNarrow(t *testing.T) {
	area := Rect{Left: 0, Top: 0, Right: 60, Bottom: 40}
	rects := hourBarRects(area, 1)
	for i, r := range rects {
		if r.Width() < 1 {
			t.Fatalf("第 %d 根柱宽度 = %d，期望 >= 1", i, r.Width())
		}
		if r.Right > area.Right {
			t.Fatalf("第 %d 根柱右侧 = %d 越界（区域右边界 %d）", i, r.Right, area.Right)
		}
		if i > 0 && r.Left < rects[i-1].Right {
			t.Fatalf("第 %d 根柱与前一根重叠", i)
		}
	}
}

// 横轴每 3 小时一个刻度：0/3/6/…/21，共 8 个。
func TestHourTickLabels(t *testing.T) {
	labels := hourTickLabels()
	want := []string{"0", "3", "6", "9", "12", "15", "18", "21"}
	if len(labels) != len(want) {
		t.Fatalf("刻度数量 = %d，期望 %d：%v", len(labels), len(want), labels)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("第 %d 个刻度 = %q，期望 %q", i, labels[i], want[i])
		}
	}
}

// 列表行矩形必须铺满可用高度且互不重叠，否则 10 行里会有一行被压扁或叠在一起。
func TestOverviewRowRects(t *testing.T) {
	area := Rect{Left: 20, Top: 240, Right: 442, Bottom: 498}
	rects := overviewRowRects(area, 10)
	if len(rects) != 10 {
		t.Fatalf("行数 = %d，期望 10", len(rects))
	}
	if rects[0].Top != area.Top {
		t.Fatalf("首行顶部 = %d，期望 %d", rects[0].Top, area.Top)
	}
	if rects[9].Bottom != area.Bottom {
		t.Fatalf("末行底部 = %d，期望 %d", rects[9].Bottom, area.Bottom)
	}
	for i, r := range rects {
		if r.Height() < 1 {
			t.Fatalf("第 %d 行高度 = %d，期望 >= 1", i, r.Height())
		}
		if r.Left != area.Left || r.Right != area.Right {
			t.Fatalf("第 %d 行左右 = %d/%d，期望与区域一致 %d/%d", i, r.Left, r.Right, area.Left, area.Right)
		}
		if i > 0 && r.Top < rects[i-1].Bottom {
			t.Fatalf("第 %d 行与上一行重叠：%+v / %+v", i, rects[i-1], r)
		}
	}

	if got := overviewRowRects(area, 0); got != nil {
		t.Fatalf("行数为 0 时应返回 nil，得到 %v", got)
	}
	if got := overviewRowRects(Rect{}, 5); got != nil {
		t.Fatalf("空区域应返回 nil，得到 %v", got)
	}
}

// 概览页骨架：四张等宽卡一行 → 两个列表并排 → 底部整宽 24 小时图。
// 900×680 客户区下这些区域必须都在 content 里、彼此不重叠，且宽度是整数除法后的等宽。
func TestComputeOverviewLayout(t *testing.T) {
	content := Rect{Left: 0, Top: 96, Right: 900, Bottom: 656}
	l := ComputeOverviewLayout(content, 1)

	pad := int32(20)
	inner := Rect{content.Left + pad, content.Top + pad, content.Right - pad, content.Bottom - pad}
	if l.Cards[0].Left != inner.Left {
		t.Fatalf("首卡左侧 = %d，期望 %d", l.Cards[0].Left, inner.Left)
	}
	if l.Cards[0].Top != inner.Top {
		t.Fatalf("首卡顶部 = %d，期望 %d", l.Cards[0].Top, inner.Top)
	}
	cardW := l.Cards[0].Width()
	if cardW <= 0 {
		t.Fatalf("卡片宽度 = %d，期望为正", cardW)
	}
	for i := 1; i < 4; i++ {
		if l.Cards[i].Width() != cardW {
			t.Fatalf("第 %d 张卡宽度 = %d，期望与首卡等宽 %d", i+1, l.Cards[i].Width(), cardW)
		}
		if l.Cards[i].Top != l.Cards[0].Top || l.Cards[i].Bottom != l.Cards[0].Bottom {
			t.Fatalf("第 %d 张卡高度/位置与首卡不一致：%+v vs %+v", i+1, l.Cards[i], l.Cards[0])
		}
		gap := l.Cards[i].Left - l.Cards[i-1].Right
		if gap < int32(12) {
			t.Fatalf("第 %d/%d 张卡间距 = %d，期望 >= 12", i, i+1, gap)
		}
	}
	if l.Cards[3].Right > inner.Right {
		t.Fatalf("末卡右侧 = %d，越出内容区 %d", l.Cards[3].Right, inner.Right)
	}
	if inner.Right-l.Cards[3].Right >= 24 {
		t.Fatalf("右侧空白 %d px，卡片宽度分配过于保守", inner.Right-l.Cards[3].Right)
	}

	// 列表并排、等高，且紧接卡片下方。
	if l.Keys.Top != l.Combos.Top || l.Keys.Bottom != l.Combos.Bottom {
		t.Fatalf("两个列表未对齐：keys=%+v combos=%+v", l.Keys, l.Combos)
	}
	if l.Keys.Top <= l.Cards[0].Bottom {
		t.Fatalf("列表顶部 %d 应低于卡片底部 %d", l.Keys.Top, l.Cards[0].Bottom)
	}
	if l.Keys.Right >= l.Combos.Left {
		t.Fatalf("两个列表重叠：keys.right=%d combos.left=%d", l.Keys.Right, l.Combos.Left)
	}
	if l.Combos.Right != inner.Right {
		t.Fatalf("右侧列表右边界 = %d，期望贴住内容区 %d", l.Combos.Right, inner.Right)
	}

	// 24 小时图整宽贴底，且与列表之间留出间距。
	if l.Hours.Left != inner.Left || l.Hours.Right != inner.Right {
		t.Fatalf("时段图未横向铺满：%+v", l.Hours)
	}
	if l.Hours.Bottom != inner.Bottom {
		t.Fatalf("时段图底部 = %d，期望 %d", l.Hours.Bottom, inner.Bottom)
	}
	if l.Hours.Top <= l.Keys.Bottom {
		t.Fatalf("时段图顶部 %d 应低于列表底部 %d", l.Hours.Top, l.Keys.Bottom)
	}

	// Body 是空状态文案的落点：要覆盖列表 + 时段图，但不含指标卡。
	if l.Body.Top != l.Keys.Top || l.Body.Bottom != l.Hours.Bottom {
		t.Fatalf("Body = %+v，期望覆盖 %d..%d", l.Body, l.Keys.Top, l.Hours.Bottom)
	}

	for name, r := range map[string]Rect{
		"卡片1": l.Cards[0], "卡片2": l.Cards[1], "卡片3": l.Cards[2], "卡片4": l.Cards[3],
		"按键列表": l.Keys, "组合列表": l.Combos, "时段图": l.Hours,
	} {
		if r.Empty() {
			t.Fatalf("%s 区域为空：%+v", name, r)
		}
		if r.Left < inner.Left || r.Right > inner.Right || r.Top < inner.Top || r.Bottom > inner.Bottom {
			t.Fatalf("%s 越出内容区：%+v（内容区 %+v）", name, r, inner)
		}
	}
}

// 最小客户区与 200% 缩放下也要给出「非空、不越界、不互相压住」的布局。
func TestComputeOverviewLayoutScales(t *testing.T) {
	cases := []struct {
		name    string
		content Rect
		scale   float64
	}{
		{"800x600 最小客户区", Rect{Left: 0, Top: 96, Right: 800, Bottom: 576}, 1},
		{"1800x1360 200%", Rect{Left: 0, Top: 192, Right: 1800, Bottom: 1312}, 2},
		{"1125x850 125%", Rect{Left: 0, Top: 120, Right: 1125, Bottom: 820}, 1.25},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := ComputeOverviewLayout(c.content, c.scale)
			rects := []Rect{l.Cards[0], l.Cards[1], l.Cards[2], l.Cards[3], l.Keys, l.Combos, l.Hours}
			for i, r := range rects {
				if r.Empty() {
					t.Fatalf("第 %d 个区域为空：%+v", i, r)
				}
				if r.Left < c.content.Left || r.Right > c.content.Right ||
					r.Top < c.content.Top || r.Bottom > c.content.Bottom {
					t.Fatalf("第 %d 个区域越出内容区：%+v", i, r)
				}
				for j := 0; j < i; j++ {
					other := rects[j]
					overlap := r.Left < other.Right && other.Left < r.Right && r.Top < other.Bottom && other.Top < r.Bottom
					if overlap {
						t.Fatalf("区域 %d 与 %d 重叠：%+v / %+v", j, i, other, r)
					}
				}
			}
		})
	}
}

// 区块标题 + 内容的两段切分：标题在上，内容剩余高度，边界严丝合缝。
func TestOverviewSection(t *testing.T) {
	area := Rect{Left: 20, Top: 216, Right: 442, Bottom: 496}
	title, body := overviewSection(area, 1)
	if title.Top != area.Top || title.Left != area.Left || title.Right != area.Right {
		t.Fatalf("标题区 = %+v，期望贴住区域上沿并横向铺满", title)
	}
	if title.Height() != 22 {
		t.Fatalf("标题高度 = %d，期望 22", title.Height())
	}
	if body.Top != title.Bottom {
		t.Fatalf("内容区顶部 = %d，期望等于标题底部 %d", body.Top, title.Bottom)
	}
	if body.Bottom != area.Bottom || body.Left != area.Left || body.Right != area.Right {
		t.Fatalf("内容区 = %+v，期望填满区域剩余部分 %+v", body, area)
	}
}

// 时段图内部三段：标题 / 柱状区 / 刻度区，顺序与包含关系都要成立。
func TestOverviewHourChart(t *testing.T) {
	area := Rect{Left: 20, Top: 512, Right: 880, Bottom: 644}
	title, bars, ticks := overviewHourChart(area, 1)
	if title.Bottom > bars.Top {
		t.Fatalf("标题底部 %d 压到了柱状区顶部 %d", title.Bottom, bars.Top)
	}
	if bars.Bottom > ticks.Top {
		t.Fatalf("柱状区底部 %d 不能压到刻度区顶部 %d", bars.Bottom, ticks.Top)
	}
	if ticks.Bottom != area.Bottom {
		t.Fatalf("刻度区底部 = %d，期望 %d", ticks.Bottom, area.Bottom)
	}
	if ticks.Height() != 18 {
		t.Fatalf("刻度区高度 = %d，期望 18", ticks.Height())
	}
	if bars.Height() <= 0 {
		t.Fatalf("柱状区高度 = %d，期望为正", bars.Height())
	}
	if bars.Left != area.Left || bars.Right != area.Right {
		t.Fatalf("柱状区未横向铺满：%+v", bars)
	}
}

// 空状态判定与文案：只有 0（或异常负数）才算「没有数据」。
func TestOverviewEmpty(t *testing.T) {
	if !overviewEmpty(0) {
		t.Fatal("0 次按键应判定为空状态")
	}
	if !overviewEmpty(-3) {
		t.Fatal("负数（脏数据）应判定为空状态")
	}
	if overviewEmpty(1) {
		t.Fatal("1 次按键不应判定为空状态")
	}
}

// 空状态与服务区空提示必须给出可读的中文文案，不能是空串。
func TestOverviewTexts(t *testing.T) {
	title, hint := overviewEmptyText()
	if title == "" || hint == "" {
		t.Fatalf("空状态文案为空：title=%q hint=%q", title, hint)
	}
	if !strings.Contains(title, "还没有数据") {
		t.Fatalf("空状态标题 = %q，应包含「还没有数据」", title)
	}
	if !strings.Contains(hint, "TOP 10") || !strings.Contains(hint, "24 小时") {
		t.Fatalf("空状态提示 = %q，应说明会显示哪些内容", hint)
	}

	cTitle, cHint := overviewComboEmptyText()
	if cTitle == "" || cHint == "" {
		t.Fatalf("组合键空提示为空：title=%q hint=%q", cTitle, cHint)
	}
	if !strings.Contains(cTitle, "组合键") {
		t.Fatalf("组合键空标题 = %q，应提到组合键", cTitle)
	}

	if got := overviewHourEmptyText(); got == "" {
		t.Fatal("时段分布空提示不能为空串")
	}
}
