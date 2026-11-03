package tray

import "testing"

// labels 把菜单项文案抽出来，方便逐项比对（分隔线用空串占位）。
func labels(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Label
	}
	return out
}

func sameLabels(got []Item, want []string) bool {
	g := labels(got)
	if len(g) != len(want) {
		return false
	}
	for i := range g {
		if g[i] != want[i] {
			return false
		}
	}
	return true
}

func TestMenuShowsPauseWhenCollecting(t *testing.T) {
	items := Menu(true, true)

	want := []string{"显示主窗口", "", "暂停采集", "关闭窗口时收进托盘", "", "退出"}
	if !sameLabels(items, want) {
		t.Fatalf("菜单项 = %q，期望 %q", labels(items), want)
	}
	if items[2].Label != "暂停采集" {
		t.Error("采集中时第 3 项应是「暂停采集」")
	}
	if !items[1].Separator || !items[4].Separator {
		t.Error("第 2、5 项应是分隔线")
	}
	for _, i := range []int{0, 2, 3, 5} {
		if items[i].Separator {
			t.Errorf("第 %d 项不该是分隔线", i+1)
		}
	}
}

func TestMenuShowsResumeWhenPaused(t *testing.T) {
	items := Menu(false, true)

	if items[2].Label != "继续采集" {
		t.Errorf("暂停时第 3 项应变成「继续采集」，实际 %q", items[2].Label)
	}
	// 暂停时这一项应该能一眼看出当前状态
	if !items[2].Checked {
		t.Error("「继续采集」应标记 Checked，表示当前处于暂停状态")
	}
	if Menu(true, true)[2].Checked {
		t.Error("采集中的「暂停采集」不该是 Checked")
	}
}

func TestMenuChecksCloseToTray(t *testing.T) {
	// 托盘菜单是改关窗行为的地方（设置页里也有），勾选状态必须反映真实设置，
	// 否则用户改完看不出结果。
	if !Menu(true, true)[3].Checked {
		t.Error("关闭窗口时收进托盘 = 开时，该项应打勾")
	}
	if Menu(true, false)[3].Checked {
		t.Error("关闭窗口时收进托盘 = 关时，该项不该打勾")
	}
	if Menu(true, false)[3].Label != "关闭窗口时收进托盘" {
		t.Errorf("勾选项的文案不该随状态变化（那样会看不出这是个开关），实际 %q", Menu(true, false)[3].Label)
	}
}

func TestMenuIDsAreStable(t *testing.T) {
	// 这些 ID 会被窗口过程用来分发命令，改动等于改协议；
	// 更要紧的是别出现重复 ID，否则点「退出」可能触发别的动作。
	seen := map[uint32]bool{}
	for _, it := range Menu(true, true) {
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
	if !seen[IDShow] || !seen[IDToggle] || !seen[IDExit] || !seen[IDCloseToTray] {
		t.Errorf("所有动作的 ID 都必须在菜单里出现，实际: %v", seen)
	}
	if IDShow != 1 || IDToggle != 2 || IDExit != 3 || IDCloseToTray != 4 {
		t.Errorf("ID 值变了（%d/%d/%d/%d），确认没有依赖它们的外部代码后再改",
			IDShow, IDToggle, IDExit, IDCloseToTray)
	}
}

func TestActionFor(t *testing.T) {
	cases := []struct {
		name   string
		id     uint32
		want   Action
		wantOk bool
	}{
		{"显示窗口", IDShow, ActionShow, true},
		{"切换采集", IDToggle, ActionToggleCollect, true},
		{"切换关窗行为", IDCloseToTray, ActionToggleCloseToTray, true},
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

func TestCloseChoiceTexts(t *testing.T) {
	// 首次关窗时弹的选项框：三行必须把「是/否/取消」分别对应什么说清楚，
	// 否则用户只能靠猜（系统按钮只有是/否/取消）。
	title, text := CloseChoice()
	if title == "" || text == "" {
		t.Fatal("关窗选项的标题和正文都不能为空")
	}
	for _, want := range []string{"收进托盘", "退出", "取消"} {
		if !contains(text, want) {
			t.Errorf("选项说明里应提到「%s」，实际:\n%s", want, text)
		}
	}
	if !contains(text, "托盘") || !contains(text, "菜单") {
		t.Error("要告诉用户以后在哪儿改这个选择（提到托盘处的菜单）")
	}
}

func TestQuitMessageName(t *testing.T) {
	if QuitMessageName == "" {
		t.Fatal("升级握手消息名不能为空，否则安装器发出去没人接")
	}
	// 这个名字要和 scripts/installer.iss 里的一字不差，改了两边都得改
	if QuitMessageName != "KeyboardStatistics.QuitForUpgrade" {
		t.Errorf("握手消息名变了（%q）；同时更新 scripts/installer.iss 的 QuitForUpgradeMsg", QuitMessageName)
	}
}

func TestAutostartHintDiffersFromHiddenHint(t *testing.T) {
	at, ab := AutostartHint()
	ht, hb := HiddenHint()

	if at == "" || ab == "" {
		t.Fatal("自启提示的标题和内容都不能为空")
	}
	if at == ht || ab == hb {
		t.Error("开机自启（窗口从没出现过）与手动关窗（窗口刚被收起）是两种场景，文案要区分开")
	}
	// 自启这条是「采集已经开始了」的唯一告知，必须说到后台与托盘
	// （气泡的标题和正文会一起显示，所以两者合起来判断）
	whole := at + ab
	if !contains(whole, "后台") || !contains(whole, "托盘") {
		t.Errorf("自启提示应说明在后台运行、入口在托盘，实际标题 %q 正文 %q", at, ab)
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
