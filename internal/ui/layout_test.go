package ui

import "testing"

func TestRectContains(t *testing.T) {
	r := Rect{Left: 10, Top: 20, Right: 110, Bottom: 70}

	cases := []struct {
		name       string
		x, y       int32
		wantInside bool
	}{
		{"左上角内", 10, 20, true},
		{"右下角外（右下开区间）", 110, 70, false},
		{"右下角内侧", 109, 69, true},
		{"中心", 60, 45, true},
		{"左侧越界", 9, 45, false},
		{"上方越界", 60, 19, false},
		{"右侧越界", 111, 45, false},
		{"下方越界", 60, 71, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := r.Contains(c.x, c.y); got != c.wantInside {
				t.Fatalf("Rect%+v.Contains(%d,%d)=%v，期望 %v", r, c.x, c.y, got, c.wantInside)
			}
		})
	}
}

func TestRectEmptyAndSize(t *testing.T) {
	r := Rect{Left: 5, Top: 6, Right: 25, Bottom: 16}
	if r.Width() != 20 || r.Height() != 10 {
		t.Fatalf("Width/Height = %d/%d，期望 20/10", r.Width(), r.Height())
	}
	if (Rect{}).Empty() != true {
		t.Fatal("零值 Rect 应当是 Empty")
	}
	if r.Empty() {
		t.Fatal("非空 Rect 不应当 Empty")
	}
}

// 三段式布局：工具栏在最上、标签栏居中、状态栏贴底，内容区填满剩余空间，互不重叠。
func TestComputeLayoutBars(t *testing.T) {
	cases := []struct {
		name   string
		client Rect
		scale  float64
	}{
		{"900x680 标准", Rect{Right: 900, Bottom: 680}, 1},
		{"800x600 最小", Rect{Right: 800, Bottom: 600}, 1},
		{"940x720", Rect{Right: 940, Bottom: 720}, 1},
		{"1800x1360 200% 缩放", Rect{Right: 1800, Bottom: 1360}, 2},
		{"1125x850 125% 缩放", Rect{Right: 1125, Bottom: 850}, 1.25},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := ComputeLayout(c.client, c.scale, 4, 3)
			cl := c.client

			if l.Toolbar.Top != cl.Top || l.Toolbar.Left != cl.Left || l.Toolbar.Right != cl.Right {
				t.Fatalf("工具栏应当贴住客户区上方并横向铺满，得到 %+v，client=%+v", l.Toolbar, cl)
			}
			if l.Toolbar.Height() <= 0 {
				t.Fatalf("工具栏高度必须为正，得到 %d", l.Toolbar.Height())
			}
			if l.TabBar.Top != l.Toolbar.Bottom {
				t.Fatalf("标签栏应紧接工具栏下方：tabbar.top=%d toolbar.bottom=%d", l.TabBar.Top, l.Toolbar.Bottom)
			}
			if l.TabBar.Height() <= 0 {
				t.Fatalf("标签栏高度必须为正，得到 %d", l.TabBar.Height())
			}
			if l.Content.Top != l.TabBar.Bottom {
				t.Fatalf("内容区应从标签栏下方开始：content.top=%d tabbar.bottom=%d", l.Content.Top, l.TabBar.Bottom)
			}
			if l.StatusBar.Bottom != cl.Bottom {
				t.Fatalf("状态栏应贴住客户区底部：statusbar.bottom=%d client.bottom=%d", l.StatusBar.Bottom, cl.Bottom)
			}
			if l.Content.Bottom != l.StatusBar.Top {
				t.Fatalf("内容区应止于状态栏上方：content.bottom=%d statusbar.top=%d", l.Content.Bottom, l.StatusBar.Top)
			}
			if l.Content.Height() <= 0 || l.Content.Width() != cl.Width() {
				t.Fatalf("内容区尺寸异常：%+v", l.Content)
			}
		})
	}
}

// 时间范围按钮：数量与入参一致、从左到右排列不重叠、完全落在工具栏内。
func TestComputeLayoutScopes(t *testing.T) {
	cases := []struct {
		name   string
		client Rect
		scale  float64
		count  int
	}{
		{"900x680 四个范围", Rect{Right: 900, Bottom: 680}, 1, 4},
		{"800x600 四个范围", Rect{Right: 800, Bottom: 600}, 1, 4},
		{"1800x1360 200% 缩放", Rect{Right: 1800, Bottom: 1360}, 2, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := ComputeLayout(c.client, c.scale, c.count, 3)
			if len(l.Scopes) != c.count {
				t.Fatalf("范围按钮数 = %d，期望 %d", len(l.Scopes), c.count)
			}
			for i, r := range l.Scopes {
				if r.Width() <= 0 || r.Height() <= 0 {
					t.Fatalf("第 %d 个范围按钮尺寸非法：%+v", i, r)
				}
				if r.Top < l.Toolbar.Top || r.Bottom > l.Toolbar.Bottom {
					t.Fatalf("第 %d 个范围按钮越出工具栏：%+v toolbar=%+v", i, r, l.Toolbar)
				}
				if r.Left < c.client.Left || r.Right > c.client.Right {
					t.Fatalf("第 %d 个范围按钮越出客户区：%+v", i, r)
				}
				if i > 0 && r.Left < l.Scopes[i-1].Right {
					t.Fatalf("范围按钮 %d 与 %d 重叠：%+v %+v", i-1, i, l.Scopes[i-1], r)
				}
			}
		})
	}
}

// 标签按钮：数量与入参一致、从左到右、不重叠、落在标签栏内。
func TestComputeLayoutTabs(t *testing.T) {
	cases := []struct {
		name   string
		client Rect
		scale  float64
		count  int
	}{
		{"900x680 三页", Rect{Right: 900, Bottom: 680}, 1, 3},
		{"1800x1360 三页", Rect{Right: 1800, Bottom: 1360}, 2, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := ComputeLayout(c.client, c.scale, 4, c.count)
			if len(l.Tabs) != c.count {
				t.Fatalf("标签数 = %d，期望 %d", len(l.Tabs), c.count)
			}
			for i, r := range l.Tabs {
				if r.Width() <= 0 || r.Height() <= 0 {
					t.Fatalf("第 %d 个标签尺寸非法：%+v", i, r)
				}
				if r.Top < l.TabBar.Top || r.Bottom > l.TabBar.Bottom {
					t.Fatalf("第 %d 个标签越出标签栏：%+v tabbar=%+v", i, r, l.TabBar)
				}
				if i > 0 && r.Left < l.Tabs[i-1].Right {
					t.Fatalf("标签 %d 与 %d 重叠", i-1, i)
				}
			}
		})
	}
}

func TestLayoutHitScope(t *testing.T) {
	l := ComputeLayout(Rect{Right: 900, Bottom: 680}, 1, 4, 3)

	for i, r := range l.Scopes {
		cx, cy := (r.Left+r.Right)/2, (r.Top+r.Bottom)/2
		got, ok := l.HitScope(cx, cy)
		if !ok || got != i {
			t.Fatalf("点击第 %d 个范围按钮中心 (%d,%d) 得到 (%d,%v)，期望 (%d,true)", i, cx, cy, got, ok, i)
		}
	}

	misses := []struct {
		name string
		x, y int32
	}{
		{"内容区", l.Content.Left + 10, l.Content.Top + 10},
		{"状态栏", l.Client.Left + 5, l.StatusBar.Bottom - 2},
		{"工具栏底部缝隙", 450, l.Toolbar.Bottom - 1},
		{"客户区外", -5, -5},
	}
	for _, m := range misses {
		if _, ok := l.HitScope(m.x, m.y); ok {
			t.Fatalf("%s (%d,%d) 不应命中时间范围按钮", m.name, m.x, m.y)
		}
	}
}

func TestLayoutHitTab(t *testing.T) {
	l := ComputeLayout(Rect{Right: 900, Bottom: 680}, 1, 4, 3)

	for i, r := range l.Tabs {
		cx, cy := (r.Left+r.Right)/2, (r.Top+r.Bottom)/2
		got, ok := l.HitTab(cx, cy)
		if !ok || got != i {
			t.Fatalf("点击第 %d 个标签中心 (%d,%d) 得到 (%d,%v)，期望 (%d,true)", i, cx, cy, got, ok, i)
		}
	}

	if _, ok := l.HitTab(l.Content.Left+10, l.Content.Top+10); ok {
		t.Fatal("内容区不应命中标签按钮")
	}
	if _, ok := l.HitTab(l.Scopes[0].Left+2, l.Scopes[0].Top+2); ok {
		t.Fatal("工具栏不应命中标签按钮")
	}
}

// 设置页各控件必须落在内容区内、自上而下排布、互不重叠，命中测试与位置一致。
func TestComputeSettingsLayout(t *testing.T) {
	cases := []struct {
		name   string
		client Rect
		scale  float64
	}{
		{"900x680", Rect{Right: 900, Bottom: 680}, 1},
		{"800x600 最小", Rect{Right: 800, Bottom: 600}, 1},
		{"1800x1360 200%", Rect{Right: 1800, Bottom: 1360}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := ComputeLayout(c.client, c.scale, 4, 3)
			s := ComputeSettingsLayout(l.Content, c.scale)

			interactive := []struct {
				name string
				r    Rect
				hit  SettingsHit
			}{
				{"采集开关", s.CollectBtn, SettingsHitCollect},
				{"自启复选框", s.AutostartBox, SettingsHitAutostart},
				{"导出 CSV", s.ExportBtn, SettingsHitExport},
				{"清空数据", s.ClearBtn, SettingsHitClear},
				{"打开数据目录", s.OpenDirBtn, SettingsHitOpenDataDir},
				{"更改数据目录", s.ChangeDirBtn, SettingsHitChangeDataDir},
			}

			for _, it := range interactive {
				if it.r.Empty() {
					t.Fatalf("%s 尺寸为零", it.name)
				}
				if it.r.Left < l.Content.Left || it.r.Right > l.Content.Right ||
					it.r.Top < l.Content.Top || it.r.Bottom > l.Content.Bottom {
					t.Fatalf("%s 越出内容区：%+v content=%+v", it.name, it.r, l.Content)
				}
				cx, cy := (it.r.Left+it.r.Right)/2, (it.r.Top+it.r.Bottom)/2
				if got := s.Hit(cx, cy); got != it.hit {
					t.Fatalf("点击 %s 中心得到 %v，期望 %v", it.name, got, it.hit)
				}
			}

			for i := 0; i < len(interactive); i++ {
				for j := i + 1; j < len(interactive); j++ {
					if rectOverlap(interactive[i].r, interactive[j].r) {
						t.Fatalf("%s 与 %s 重叠：%+v %+v", interactive[i].name, interactive[j].name, interactive[i].r, interactive[j].r)
					}
				}
			}

			// 自上而下：采集 → 自启 → 数据操作 → 关于
			if !(s.CollectBtn.Bottom <= s.AutostartBox.Top) {
				t.Fatalf("采集开关应在自启复选框之上：%+v %+v", s.CollectBtn, s.AutostartBox)
			}
			if !(s.AutostartBox.Bottom <= s.ExportBtn.Top) {
				t.Fatalf("自启复选框应在数据操作之上：%+v %+v", s.AutostartBox, s.ExportBtn)
			}
			if !(s.ExportBtn.Bottom <= s.OpenDirBtn.Top) {
				t.Fatalf("数据操作应在关于之上：%+v %+v", s.ExportBtn, s.OpenDirBtn)
			}
			if s.AboutTop <= s.OpenDirBtn.Bottom {
				t.Fatalf("关于区块应位于打开目录按钮下方：aboutTop=%d openDirBottom=%d", s.AboutTop, s.OpenDirBtn.Bottom)
			}
			// 「打开数据目录」和「更改数据目录」同一行，顺序固定，别在缩放后叠起来
			if s.ChangeDirBtn.Left <= s.OpenDirBtn.Right {
				t.Fatalf("更改目录按钮应在打开目录按钮右侧：open=%+v change=%+v", s.OpenDirBtn, s.ChangeDirBtn)
			}
			if s.ChangeDirBtn.Right > l.Content.Right {
				t.Fatalf("更改目录按钮越出内容区：%+v content=%+v", s.ChangeDirBtn, l.Content)
			}

			// 空白处不命中任何控件
			if got := s.Hit(l.Content.Right-2, l.Content.Top+2); got != SettingsHitNone {
				t.Fatalf("内容区右上角应无命中，得到 %v", got)
			}
			if got := s.Hit(-10, -10); got != SettingsHitNone {
				t.Fatalf("客户区外应无命中，得到 %v", got)
			}
		})
	}
}

// 缩放后控件尺寸必须等比放大，否则高分屏上点不中、字挤在一起。
func TestComputeSettingsLayoutScales(t *testing.T) {
	base := ComputeSettingsLayout(Rect{Right: 900, Bottom: 680}, 1)
	double := ComputeSettingsLayout(Rect{Right: 1800, Bottom: 1360}, 2)

	if double.CollectBtn.Height() != 2*base.CollectBtn.Height() {
		t.Fatalf("2 倍缩放后按钮高度 = %d，期望 %d", double.CollectBtn.Height(), 2*base.CollectBtn.Height())
	}
	if double.CollectBtn.Width() != 2*base.CollectBtn.Width() {
		t.Fatalf("2 倍缩放后按钮宽度 = %d，期望 %d", double.CollectBtn.Width(), 2*base.CollectBtn.Width())
	}
	if double.AutostartBox.Height() != 2*base.AutostartBox.Height() {
		t.Fatalf("2 倍缩放后复选框高度 = %d，期望 %d", double.AutostartBox.Height(), 2*base.AutostartBox.Height())
	}
}

func rectOverlap(a, b Rect) bool {
	if a.Empty() || b.Empty() {
		return false
	}
	return a.Left < b.Right && b.Left < a.Right && a.Top < b.Bottom && b.Top < a.Bottom
}
