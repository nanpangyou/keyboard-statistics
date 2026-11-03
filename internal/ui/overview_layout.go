package ui

import (
	"strconv"
	"strings"

	"keyboard-statistics/internal/stats"
)

// 本文件是概览页里不碰 Win32 的部分：数值格式化、指标卡文案、TOP 列表行、
// 24 小时柱高换算与区块矩形分布。全部是纯函数，能在任何平台用表驱动测试兜住
// （page_overview.go 只负责按这些结果填色填字）。
//
// 口径一律以 docs/metrics.md 为准，这里不做任何「看起来更合理」的二次加工。

// 概览页在 96 DPI 下的基准尺寸，使用前统一乘 DPI 比例（px）。
// 名字统一带 Ov 前缀，避免和 layout.go 里的设置页尺寸撞名。
const (
	baseOvPad        = 20
	baseOvGapY       = 16
	baseOvColGap     = 16
	baseOvCardGap    = 12
	baseOvCardH      = 92
	baseOvCardPadX   = 14
	baseOvCardPadY   = 10
	baseOvCardTitleH = 18
	baseOvCardValueH = 30
	baseOvValueSize  = 24

	baseOvTitleH    = 22
	baseOvChartH    = 132
	baseOvTickH     = 18
	baseOvTickW     = 30
	baseOvTickGap   = 4
	baseOvBarGap    = 5
	baseOvMaxLabelH = 16
	baseOvBarIdleH  = 2

	baseOvRankW    = 20
	baseOvCountW   = 64
	baseOvPercentW = 46
	// baseOvRankGap 是排名数字与键名之间的空档。
	// 这个值不是装饰：贴着画时「1空格」「2E」会被读成一个词，榜单就没法扫读了。
	baseOvRankGap  = 12
	baseOvRowGap   = 14
	baseOvRowInset = 4
	// baseOvBarPadY 是行内比例条上下留的空，让条看起来是「一行的一部分」而不是一条下划线。
	baseOvBarPadY = 6
)

// overviewTopN 是两张 TOP 榜的行数上限（docs/metrics.md §5）。
const overviewTopN = 10

// formatKeyCount 把按键次数格式化成卡片宽度放得下的短形式：
// 一万以下是千分位（1,234），一万到一亿是「万」，一亿以上是「亿」，最多带两位小数。
//
// 不用 float64：次数是 int64 精确统计，转浮点再四舍五入在边界上会抖
// （比如 99999 变成 10.0 万还是 9.9 万），这里一律整数运算。
func formatKeyCount(n int64) string {
	if n < 0 {
		n = 0
	}
	switch {
	case n < 10000:
		return groupThousands(n)
	case n < 100000000:
		// 千分之一万 = 0.1 万，即最小刻度是 1000。
		return trimDecimals((n+500)/1000, 10) + " 万"
	default:
		// 百万分之一亿 = 0.01 亿，即最小刻度是 100 万。
		return trimDecimals((n+500000)/1000000, 100) + " 亿"
	}
}

// groupThousands 按三位一组加千分位：1234567 → 1,234,567。
func groupThousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	lead := len(s) % 3
	if len(s) <= 3 {
		return s
	}

	var b strings.Builder
	if lead > 0 {
		b.WriteString(s[:lead])
	}
	for i := lead; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// trimDecimals 把「放大过的小数」还原成人类可读的小数并去掉末尾多余的 0：
// scaled=1235、unit=10 → 123.5；scaled=200、unit=100 → 2。
func trimDecimals(scaled, unit int64) string {
	whole := scaled / unit
	frac := scaled % unit
	if frac == 0 {
		return strconv.FormatInt(whole, 10)
	}

	width := 0
	for u := unit; u > 1; u /= 10 {
		width++
	}
	s := strconv.FormatInt(frac, 10)
	if pad := width - len(s); pad > 0 {
		s = strings.Repeat("0", pad) + s
	}
	s = strings.TrimRight(s, "0")
	return strconv.FormatInt(whole, 10) + "." + s
}

// formatActiveDuration 按 docs/metrics.md §8 显示活跃时长：
// 「3 小时 12 分」；不足 1 小时只写分钟；不足 1 分钟写「不足 1 分」。
func formatActiveDuration(sec int64) string {
	if sec < 0 {
		sec = 0
	}
	if sec < 60 {
		return "不足 1 分"
	}

	minutes := sec / 60
	if minutes < 60 {
		return strconv.FormatInt(minutes, 10) + " 分"
	}

	hours := minutes / 60
	rest := minutes % 60
	if rest == 0 {
		return strconv.FormatInt(hours, 10) + " 小时"
	}
	return strconv.FormatInt(hours, 10) + " 小时 " + strconv.FormatInt(rest, 10) + " 分"
}

// formatMonthDay 把库里的 YYYY-MM-DD 压成卡片副标题用的「月-日」。
// 格式不对（脏数据、空串）返回空串，宁可副标题留白也不在界面上漏半截日期。
func formatMonthDay(day string) string {
	if len(day) != 10 || day[4] != '-' || day[7] != '-' {
		return ""
	}
	for i := 0; i < len(day); i++ {
		if i == 4 || i == 7 {
			continue
		}
		if day[i] < '0' || day[i] > '9' {
			return ""
		}
	}
	return day[5:7] + "-" + day[8:10]
}

// overviewEmpty 报告当前范围是不是「没有数据」。
// 负数只可能来自脏数据，同样按空状态处理（绝不给用户看负的按键数）。
func overviewEmpty(totalKeys int64) bool { return totalKeys <= 0 }

// overviewMetrics 是概览页用到的快照字段，单独抽出来是为了让指标卡文案能脱库测试。
type overviewMetrics struct {
	TotalKeys     int64
	ActiveSeconds int64
	DailyAverage  int64
	PeakCount     int64
	PeakDay       string
	Days          int
}

// cardSpec 是一张指标卡要画的三行：标题、大号数字、灰色副标题。
type cardSpec struct {
	Title string
	Value string
	Sub   string
}

// overviewCardSpecs 生成四张指标卡的文案，顺序即从左到右的显示顺序。
//
// 没有数据时数值一律是「—」而不是 0：0 会被读成「统计到了 0 次」，
// 和「这个范围还没有数据」是两件事。
func overviewCardSpecs(m overviewMetrics) [4]cardSpec {
	empty := overviewEmpty(m.TotalKeys)
	value := func(s string) string {
		if empty || s == "" {
			return "—"
		}
		return s
	}

	// 天数未知（范围退化）时不写「共 0 天」，那是自相矛盾的一句话。
	daysSub := ""
	if m.Days > 0 {
		daysSub = "共 " + strconv.Itoa(m.Days) + " 天"
	}

	return [4]cardSpec{
		{Title: "总按键数", Value: value(formatKeyCount(m.TotalKeys)), Sub: "不含组合键"},
		{Title: "活跃时长", Value: value(formatActiveDuration(m.ActiveSeconds)), Sub: "间隔 60 秒算同一会话"},
		{Title: "日均按键", Value: value(formatKeyCount(m.DailyAverage)), Sub: daysSub},
		{Title: "单日峰值", Value: value(formatKeyCount(m.PeakCount)), Sub: formatMonthDay(m.PeakDay)},
	}
}

// overviewRow 是 TOP 榜一行要显示的四个字段：序号 / 名称 / 次数 / 占比。
type overviewRow struct {
	Rank    string
	Name    string
	Count   string
	Percent string
	// Raw 是没格式化过的原始次数，只用来算行内比例条的宽度
	// （显示用的 Count 已经带了千分位/万/亿这种缩写，没法直接拿去算比例）。
	Raw int64
}

// overviewRowCells 把一行横向切成四格：排名 / 名称 / 次数 / 占比。
//
// 四格之间必须有明确的空档：排行榜靠的是「竖着对齐、横着留白」，
// 排名数字一旦贴住键名，读者会把两者当成一个词。
func overviewRowCells(row Rect, scale float64) (rank, name, count, percent Rect) {
	if row.Empty() {
		return Rect{}, Rect{}, Rect{}, Rect{}
	}

	rankW := px(baseOvRankW, scale)
	rankGap := px(baseOvRankGap, scale)
	countW := px(baseOvCountW, scale)
	percentW := px(baseOvPercentW, scale)
	nameGap := px(baseOvRowGap, scale)

	// 右侧两列先定位，它们贴着行的右边界右对齐。
	percent = Rect{row.Right - percentW, row.Top, row.Right, row.Bottom}
	count = Rect{percent.Left - countW, row.Top, percent.Left, row.Bottom}

	rank = Rect{row.Left, row.Top, row.Left + rankW, row.Bottom}

	nameLeft := rank.Right + rankGap
	nameRight := count.Left - nameGap
	if nameRight < nameLeft {
		// 窗口窄到放不下键名时，交一个空矩形而不是左右颠倒的矩形：
		// 颠倒的 Rect 会让 DrawText 画出看不见的乱码，比不画更难查。
		name = Rect{nameLeft, row.Top, nameLeft, row.Bottom}
		return rank, name, count, percent
	}
	name = Rect{nameLeft, row.Top, nameRight, row.Bottom}
	return rank, name, count, percent
}

// overviewRowBar 是行内那条表示相对大小的横条：宽 = 本行次数 / 榜首次数。
//
// 光看数字要在脑子里做除法，一条横条能让人一眼扫出谁高谁低。
func overviewRowBar(area Rect, count, top int64, scale float64) Rect {
	if area.Empty() {
		return Rect{}
	}
	padY := px(baseOvBarPadY, scale)
	barTop, barBottom := area.Top+padY, area.Bottom-padY
	if barBottom <= barTop {
		return Rect{}
	}
	// 注意不能拿 Rect.Empty 判断这里：条是按宽度算出来的，初始宽度就是 0，
	// Empty 会把它当成「没有」而直接丢掉。
	bar := Rect{area.Left, barTop, area.Left, barBottom}

	if top <= 0 || count <= 0 {
		return bar // 宽 0：画不出来，但也不必特殊照顾
	}

	w := int32(int64(area.Width()) * count / top)
	if w > area.Width() {
		w = area.Width() // 脏数据（本行比榜首还大）不许溢出
	}
	if w < 2 {
		w = 2 // 有次数就得看得见，1px 在浅色底上等于没有
	}
	bar.Right = bar.Left + w
	return bar
}

// overviewPercent 按 docs/metrics.md §4 显示整数百分比；分母为 0 时是 0%。
func overviewPercent(part, total int64) string {
	return strconv.Itoa(stats.Percent(part, total)) + "%"
}

// overviewKeyRows 把按键排行转成界面行。键名必须走 stats.KeyName，
// 保证同一颗键在概览、热力图、导出的 CSV 里完全一致。
func overviewKeyRows(keys []stats.KeyCount, total int64) []overviewRow {
	if len(keys) > overviewTopN {
		keys = keys[:overviewTopN]
	}
	rows := make([]overviewRow, 0, len(keys))
	for i, k := range keys {
		rows = append(rows, overviewRow{
			Rank:    strconv.Itoa(i + 1),
			Name:    stats.KeyName(k.VK),
			Count:   formatKeyCount(k.Count),
			Percent: overviewPercent(k.Count, total),
			Raw:     k.Count,
		})
	}
	return rows
}

// overviewComboRows 把组合键排行转成界面行；total 必须是「组合键总次数」，
// 与按键占比的分母互相独立（metrics.md §4）。
func overviewComboRows(combos []stats.ComboCount, total int64) []overviewRow {
	if len(combos) > overviewTopN {
		combos = combos[:overviewTopN]
	}
	rows := make([]overviewRow, 0, len(combos))
	for i, c := range combos {
		rows = append(rows, overviewRow{
			Rank:    strconv.Itoa(i + 1),
			Name:    c.Combo,
			Count:   formatKeyCount(c.Count),
			Percent: overviewPercent(c.Count, total),
			Raw:     c.Count,
		})
	}
	return rows
}

// comboTotal 是全部组合键次数之和，用作组合占比的分母。
// 负数（脏数据）不计入，否则分母可能被拉到 0 以下导致占比全是 0%。
func comboTotal(counts map[string]int64) int64 {
	var total int64
	for _, c := range counts {
		if c > 0 {
			total += c
		}
	}
	return total
}

// hourlyMax 返回 24 个小时桶里的最大值；负数按 0 处理。
func hourlyMax(hourly [24]int64) int64 {
	var max int64
	for _, c := range hourly {
		if c > max {
			max = c
		}
	}
	return max
}

// hourBarHeights 把每小时的次数换算成像素柱高（0..chartH）。
//
// 按图内最大值归一化（docs/ui-plan.md §6.2）：不用总按键数做分母，
// 否则个别高频小时会把其余柱子压成一条线。max=0 时全部返回 0——
// 这里也是对「除零」的唯一处理点，调用方据此画空状态。
// 非零计数至少留 1px，否则「有数据」和「没数据」在视觉上无法区分。
func hourBarHeights(hourly [24]int64, chartH int32) [24]int32 {
	var out [24]int32
	if chartH <= 0 {
		return out
	}

	max := hourlyMax(hourly)
	if max <= 0 {
		return out
	}

	for i, c := range hourly {
		if c <= 0 {
			continue
		}
		h := int32((c*int64(chartH) + max/2) / max)
		if h < 1 {
			h = 1
		}
		if h > chartH {
			h = chartH
		}
		out[i] = h
	}
	return out
}

// hourBarRects 返回 24 根柱子的水平位置，高度先占满整个区域，实际高度由调用方按
// hourBarHeights 从底部往上填。
//
// 区域太窄时缩小间隔而不是让柱宽变成 0：0 宽的矩形在 GDI 里什么都不画，
// 用户会以为那个小时没有数据。
func hourBarRects(area Rect, scale float64) [24]Rect {
	var out [24]Rect
	if area.Empty() {
		return out
	}

	gap := px(baseOvBarGap, scale)
	if gap*23 > area.Width()/2 {
		gap = 0
	}
	barW := (area.Width() - 23*gap) / 24
	if barW < 1 {
		barW, gap = 1, 0
	}

	for i := 0; i < 24; i++ {
		left := area.Left + int32(i)*(barW+gap)
		if left > area.Right {
			left = area.Right
		}
		right := left + barW
		if right > area.Right {
			right = area.Right
		}
		out[i] = Rect{left, area.Top, right, area.Bottom}
	}
	return out
}

// hourTickLabels 是横轴刻度：每 3 小时一个，0/3/6/…/21 共 8 个。
func hourTickLabels() []string {
	labels := make([]string, 0, 8)
	for h := 0; h < 24; h += 3 {
		labels = append(labels, strconv.Itoa(h))
	}
	return labels
}

// overviewRowRects 把一块区域均分成 n 行。
//
// 用「第 i 行顶部 = i*H/n」而不是固定行高乘以 i：整数除法会把 258px 分成
// 10 个 25px，剩下的 8px 要么被丢掉、要么堆在最后一行，行高不均看起来像错位。
func overviewRowRects(area Rect, n int) []Rect {
	if n <= 0 || area.Empty() {
		return nil
	}
	h := area.Height()
	out := make([]Rect, n)
	for i := 0; i < n; i++ {
		out[i] = Rect{
			Left:   area.Left,
			Top:    area.Top + int32(i)*h/int32(n),
			Right:  area.Right,
			Bottom: area.Top + int32(i+1)*h/int32(n),
		}
	}
	return out
}

// OverviewLayout 是概览页各区块的矩形分布。
//
// 和 Layout 一样是纯计算结果：绘制只按这里的 Rect 填色填字，
// 换分辨率或缩放比例只改这个函数。
type OverviewLayout struct {
	Content Rect

	// Cards 是顶部一行四张等宽指标卡。
	Cards [4]Rect
	// Keys / Combos 是并排的两个 TOP 10 列表（含各自的标题区）。
	Keys   Rect
	Combos Rect
	// Hours 是底部整宽的 24 小时时段分布（含标题与刻度区）。
	Hours Rect
	// Body 是列表 + 时段图的外接矩形，空状态文案画在它的中央。
	Body Rect
}

// ComputeOverviewLayout 自上而下排布概览页：指标卡 → 两个列表 → 时段分布。
func ComputeOverviewLayout(content Rect, scale float64) OverviewLayout {
	l := OverviewLayout{Content: content}
	if content.Empty() {
		return l
	}

	pad := px(baseOvPad, scale)
	inner := Rect{
		content.Left + pad, content.Top + pad,
		content.Right - pad, content.Bottom - pad,
	}
	if inner.Empty() {
		return l
	}

	// —— 指标卡：等宽四列；整数除法的余数留在最右侧，不硬拉宽最后一张，
	// 否则「等宽」这条在窄窗口下会差出好几个像素。——
	cardGap := px(baseOvCardGap, scale)
	cardH := px(baseOvCardH, scale)
	cardW := (inner.Width() - 3*cardGap) / 4
	if cardW < 1 {
		cardW = 1
	}
	for i := range l.Cards {
		left := inner.Left + int32(i)*(cardW+cardGap)
		l.Cards[i] = Rect{left, inner.Top, left + cardW, inner.Top + cardH}
	}

	gapY := px(baseOvGapY, scale)

	// —— 时段图先贴底，列表吃掉剩下的高度：窗口变矮时优先压列表，
	// 图表高度固定，柱状图不会变成一条看不出来的细线。——
	chartH := px(baseOvChartH, scale)
	l.Hours = Rect{inner.Left, inner.Bottom - chartH, inner.Right, inner.Bottom}

	listsTop := l.Cards[0].Bottom + gapY
	listsBottom := l.Hours.Top - gapY

	colGap := px(baseOvColGap, scale)
	colW := (inner.Width() - colGap) / 2
	l.Keys = Rect{inner.Left, listsTop, inner.Left + colW, listsBottom}
	l.Combos = Rect{l.Keys.Right + colGap, listsTop, inner.Right, listsBottom}
	l.Body = Rect{inner.Left, listsTop, inner.Right, inner.Bottom}

	return l
}

// overviewSection 把一个区块切成「标题区 + 内容区」两段。
func overviewSection(area Rect, scale float64) (title, body Rect) {
	if area.Empty() {
		return Rect{}, Rect{}
	}
	titleH := px(baseOvTitleH, scale)
	if titleH > area.Height() {
		titleH = area.Height()
	}
	title = Rect{area.Left, area.Top, area.Right, area.Top + titleH}
	body = Rect{area.Left, title.Bottom, area.Right, area.Bottom}
	return title, body
}

// overviewHourChart 把时段图区块切成「标题 / 柱状区 / 刻度区」三段。
// 柱状区顶部留一行放最大值标注，底部留一点间隙，柱子不贴着刻度文字。
func overviewHourChart(area Rect, scale float64) (title, bars, ticks Rect) {
	title, body := overviewSection(area, scale)
	if body.Empty() {
		return title, Rect{}, Rect{}
	}

	tickH := px(baseOvTickH, scale)
	if tickH > body.Height() {
		tickH = body.Height()
	}
	tickGap := px(baseOvTickGap, scale)

	barsBottom := body.Bottom - tickH - tickGap
	if barsBottom < body.Top {
		barsBottom = body.Top
	}
	bars = Rect{body.Left, body.Top, body.Right, barsBottom}

	ticksTop := barsBottom + tickGap
	if ticksTop > body.Bottom {
		ticksTop = body.Bottom
	}
	ticks = Rect{body.Left, ticksTop, body.Right, body.Bottom}
	return title, bars, ticks
}

// overviewEmptyText 是整页没有数据时的文案：一句主文案 + 一句说明这里将显示什么。
func overviewEmptyText() (title, hint string) {
	return "还没有数据，去敲几下键盘吧",
		"开始采集后，这里会显示总按键数、活跃时长、按键 TOP 10、快捷键 TOP 10 与 24 小时时段分布。"
}

// overviewComboEmptyText 是有单键数据、但还没记录到组合键时的提示。
// 顺手给两个例子，比干巴巴一句「暂无数据」有用。
func overviewComboEmptyText() (title, hint string) {
	return "还没有记录到组合键", "试试 Ctrl+C、Alt+Tab 这类组合，它们会单独统计。"
}

// overviewHourEmptyText 是时段分布没有可画数据时的一句话。
func overviewHourEmptyText() string {
	return "这个范围里还没有可画的时段数据"
}
