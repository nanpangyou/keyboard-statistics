package tray

import "testing"

func TestMenuShowsPauseWhenCollecting(t *testing.T) {
	items := Menu(true)

	if len(items) != 4 {
		t.Fatalf("采集中的菜单应有 4 项（显示 / 分隔线 / 暂停 / 退出），实际 %d 项: %+v", len(items), items)
	}
	want := []string{"显示主窗口", "", "暂停采集", "退出"}
	for i, w := range want {
		if items[i].Label != w {
			t.Errorf("第 %d 项 = %q，期望 %q", i, items[i].Label, w)
		}
	}
	if !items[1].Separator {
		t.Error("第 2 项应是分隔线")
	}
	if items[0].Separator || items[2].Separator || items[3].Separator {
		t.Error("只有第 2 项是分隔线，其余不该是")
	}
}

func TestMenuShowsResumeWhenPaused(t *testing.T) {
	items := Menu(false)

	if len(items) != 4 {
		t.Fatalf("暂停时的菜单应有 4 项，实际 %d 项", len(items))
	}
	if items[2].Label != "继续采集" {
		t.Errorf("暂停时第 3 项应变成「继续采集」，实际 %q", items[2].Label)
	}
	// 暂停时这一项应该能一眼看出当前状态
	if !items[2].Checked {
		t.Error("「继续采集」应标记 Checked，表示当前处于暂停状态")
	}
	if Menu(true)[2].Checked {
		t.Error("采集中的「暂停采集」不该是 Checked")
	}
}

func TestMenuIDsAreStable(t *testing.T) {
	// 这些 ID 会被窗口过程用来分发命令，改动等于改协议；
	// 更要紧的是别出现重复 ID，否则点「退出」可能触发别的动作。
	seen := map[uint32]bool{}
	for _, it := range Menu(true) {
		if it.Separator {
			continue
		}
		if it.ID == 0 {
			t.Errorf("菜单项 %q 的 ID 不能是 0（0 在菜单里表示「没选中任何项」）", it.Label)
		}
		if seen[it.ID] {
			t.Errorf("菜单项 ID 重复: %d", it.ID)
		}
		seen[it.ID] = true
	}
	if !seen[IDShow] || !seen[IDToggle] || !seen[IDExit] {
		t.Errorf("三个动作的 ID 都必须在菜单里出现: show=%v toggle=%v exit=%v",
			seen[IDShow], seen[IDToggle], seen[IDExit])
	}
	if IDShow != 1 || IDToggle != 2 || IDExit != 3 {
		t.Errorf("ID 值变了（%d/%d/%d），确认没有依赖它们的外部代码后再改", IDShow, IDToggle, IDExit)
	}
}

func TestActionFor(t *testing.T) {
	cases := []struct {
		name     string
		id       uint32
		want     Action
		wantOk   bool
	}{
		{"显示窗口", IDShow, ActionShow, true},
		{"切换采集", IDToggle, ActionToggleCollect, true},
		{"退出", IDExit, ActionExit, true},
		{"未知 ID", 99, 0, false},
		{"0 = 没选中", 0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ActionFor(c.id)
			if ok != c.wantOk {
				t.Fatalf("ok = %v，期望 %v", ok, c.wantOk)
			}
			if ok && got != c.want {
				t.Errorf("action = %v，期望 %v", got, c.want)
			}
		})
	}
}

func TestTooltipReflectsCollecting(t *testing.T) {
	on := Tooltip(true)
	off := Tooltip(false)
	if on == off {
		t.Fatal("采集与暂停的托盘提示必须不同，否则用户看不出当前状态")
	}
	// 悬停提示是唯一能说明「这个图标是什么」的地方，必须带上程序名
	if !contains(on, "键盘使用统计") || !contains(off, "键盘使用统计") {
		t.Errorf("提示里应包含程序名: %q / %q", on, off)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
