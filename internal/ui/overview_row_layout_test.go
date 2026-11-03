package ui

import "testing"

// 一行里四格的横向切分必须留出明确间距。
// 之前排名和键名是紧贴着的，屏幕上直接读成「1空格」「2E」，排行榜就废了。
func TestOverviewRowCellsLeavesGaps(t *testing.T) {
	row := Rect{Left: 0, Top: 0, Right: 400, Bottom: 30}

	rank, name, count, percent := overviewRowCells(row, 1)

	if rank.Left != row.Left {
		t.Errorf("排名格子左边界 = %d，期望 %d", rank.Left, row.Left)
	}
	if rank.Width() != baseOvRankW {
		t.Errorf("排名格子宽 = %d，期望 %d", rank.Width(), baseOvRankW)
	}

	gap := name.Left - rank.Right
	if gap != baseOvRankGap {
		t.Errorf("排名与键名间距 = %d，期望 %d", gap, baseOvRankGap)
	}
	// 这条是这次改动的核心：间距必须真的存在，不能被压成 0
	if gap < 8 {
		t.Fatalf("排名与键名间距只有 %dpx，太挤了", gap)
	}

	if gap = count.Left - name.Right; gap != baseOvRowGap {
		t.Errorf("键名与次数间距 = %d，期望 %d", gap, baseOvRowGap)
	}
	if count.Right != percent.Left {
		t.Errorf("次数右边界 = %d，占比左边界 = %d，两者应相接", count.Right, percent.Left)
	}
	if percent.Right != row.Right {
		t.Errorf("占比右边界 = %d，期望贴到行右边界 %d", percent.Right, row.Right)
	}
	if count.Width() != baseOvCountW {
		t.Errorf("次数列宽 = %d，期望 %d", count.Width(), baseOvCountW)
	}
	if percent.Width() != baseOvPercentW {
		t.Errorf("占比列宽 = %d，期望 %d", percent.Width(), baseOvPercentW)
	}
}

// 缩放后所有尺寸都要跟着走，否则高 DPI 下间距会显得比字还小。
func TestOverviewRowCellsScales(t *testing.T) {
	row := Rect{Left: 0, Top: 0, Right: 800, Bottom: 60}

	r1, n1, c1, p1 := overviewRowCells(row, 1)
	r2, n2, c2, p2 := overviewRowCells(row, 2)

	if r2.Width() != r1.Width()*2 {
		t.Errorf("2 倍缩放下排名宽 = %d，期望 %d", r2.Width(), r1.Width()*2)
	}
	if n2.Left-n1.Left != (n1.Left-r1.Right)*1 {
		t.Logf("间距随缩放变化: 1x=%d 2x=%d", n1.Left-r1.Right, n2.Left-r2.Right)
	}
	if got, want := n2.Left-r2.Right, (n1.Left-r1.Right)*2; got != want {
		t.Errorf("2 倍缩放下排名间距 = %d，期望 %d", got, want)
	}
	if got, want := c2.Left-n2.Right, (c1.Left-n1.Right)*2; got != want {
		t.Errorf("2 倍缩放下键名间距 = %d，期望 %d", got, want)
	}
	if p2.Width() != p1.Width()*2 {
		t.Errorf("2 倍缩放下占比宽 = %d，期望 %d", p2.Width(), p1.Width()*2)
	}
}

// 窗口很窄时不能让键名格子变成负数宽度（Rect 会翻过来，画出来是乱码）。
// 宁可把键名让成空，也不能交出一个左右颠倒的矩形。
func TestOverviewRowCellsNarrowRow(t *testing.T) {
	narrow := Rect{Left: 0, Top: 0, Right: 90, Bottom: 30}

	rank, name, count, percent := overviewRowCells(narrow, 1)

	if name.Width() < 0 {
		t.Fatalf("窄行下键名宽度 = %d，不能为负", name.Width())
	}
	if rank.Empty() && count.Empty() && percent.Empty() {
		t.Fatal("窄行下所有格子都空了，至少要保住排名和次数")
	}
	if !count.Empty() && count.Right > percent.Left {
		t.Fatalf("次数与占比重叠: count.Right=%d percent.Left=%d", count.Right, percent.Left)
	}
}

// 比例条按「本行占榜首的比例」给宽度，用来一眼看出高低。
func TestOverviewRowBarRatio(t *testing.T) {
	area := Rect{Left: 100, Top: 10, Right: 300, Bottom: 30}

	top := overviewRowBar(area, 100, 100, 1)
	if top.Left != area.Left {
		t.Errorf("榜首条左边 = %d，期望 %d", top.Left, area.Left)
	}
	if top.Width() != area.Width() {
		t.Errorf("榜首条宽 = %d，期望铺满 %d", top.Width(), area.Width())
	}

	half := overviewRowBar(area, 50, 100, 1)
	if got, want := half.Width(), area.Width()/2; got != want {
		t.Errorf("一半比例条宽 = %d，期望 %d", got, want)
	}
	if half.Left != area.Left {
		t.Errorf("比例条应从左边起画，实际 %d", half.Left)
	}

	// 次数为 0 或榜首为 0 时不能除零，也不能画出负宽度
	zero := overviewRowBar(area, 0, 100, 1)
	if zero.Width() < 0 {
		t.Errorf("0 次的比例条宽 = %d，不能为负", zero.Width())
	}
	if bad := overviewRowBar(area, 5, 0, 1); bad.Width() < 0 {
		t.Errorf("榜首为 0 时比例条宽 = %d，不能为负", bad.Width())
	}
}

// 占比最大也只有满格：脏数据（本行比榜首还大）不能画出溢出的条。
func TestOverviewRowBarClampsToWidth(t *testing.T) {
	area := Rect{Left: 0, Top: 0, Right: 200, Bottom: 20}

	over := overviewRowBar(area, 500, 100, 1)
	if over.Width() > area.Width() {
		t.Fatalf("超出榜首的条宽 = %d，不能超过 %d", over.Width(), area.Width())
	}
}
