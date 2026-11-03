package ui

import (
	"strings"
	"testing"
)

// 采集按钮文字必须跟着真实状态切换，否则用户点「暂停」后按钮还写着「暂停采集」。
func TestCollectButtonText(t *testing.T) {
	if got := collectButtonText(true); got != "暂停采集" {
		t.Fatalf("采集中按钮文字 = %q，期望 %q", got, "暂停采集")
	}
	if got := collectButtonText(false); got != "继续采集" {
		t.Fatalf("已暂停按钮文字 = %q，期望 %q", got, "继续采集")
	}
}

// 顶部状态点：采集中是实心绿点，暂停后必须立刻变成「已暂停」，不能停留在旧文字。
func TestStatusText(t *testing.T) {
	if got := statusText(true); got != "● 采集中" {
		t.Fatalf("采集中状态 = %q，期望 %q", got, "● 采集中")
	}
	if got := statusText(false); got != "● 已暂停" {
		t.Fatalf("已暂停状态 = %q，期望 %q", got, "● 已暂停")
	}
}

// 自启复选框的回调只应由「用户点击」触发：程序化刷新勾选状态（updating）时必须什么都不做，
// 否则每次 2 秒刷新都会往注册表写一遍，或者在注册表读失败时反复开关。
func TestAutostartAction(t *testing.T) {
	cases := []struct {
		name     string
		updating bool
		checked  bool
		want     autostartOp
	}{
		{"刷新中被勾选：不动注册表", true, true, autostartNoop},
		{"刷新中被取消勾选：不动注册表", true, false, autostartNoop},
		{"用户勾选：写入自启", false, true, autostartEnable},
		{"用户取消勾选：删除自启", false, false, autostartDisable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := autostartAction(c.updating, c.checked); got != c.want {
				t.Fatalf("autostartAction(updating=%v, checked=%v) = %v，期望 %v",
					c.updating, c.checked, got, c.want)
			}
		})
	}
}

// 自启路径失效时给一句明确的中文提示；没失效时必须为空，不能留半句残句在界面上。
func TestStaleWarningText(t *testing.T) {
	if got := staleWarningText(false); got != "" {
		t.Fatalf("未失效时提示 = %q，期望空串", got)
	}
	got := staleWarningText(true)
	if !strings.Contains(got, "自启路径已失效") {
		t.Fatalf("失效提示 = %q，应当包含「自启路径已失效」", got)
	}
}

// 底部状态栏摘要：范围、天数要能一眼看清。
func TestRangeSummary(t *testing.T) {
	cases := []struct {
		name     string
		from, to string
		days     int
		want     string
	}{
		{"今日", "2026-11-02", "2026-11-02", 1, "2026-11-02 ~ 2026-11-02 · 共 1 天"},
		{"近 7 天", "2026-10-27", "2026-11-02", 7, "2026-10-27 ~ 2026-11-02 · 共 7 天"},
		{"天数为 0 时不写天数", "2026-11-02", "2026-11-02", 0, "2026-11-02 ~ 2026-11-02"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rangeSummary(c.from, c.to, c.days); got != c.want {
				t.Fatalf("rangeSummary(%q,%q,%d) = %q，期望 %q", c.from, c.to, c.days, got, c.want)
			}
		})
	}
}

// 关于区块的文案是「隐私承诺」的一部分，缺行就是没做到，所以按行断言。
func TestAboutLines(t *testing.T) {
	lines := aboutLines("0.1.0-dev", `C:\Users\me\AppData\Local\keyboard-statistics`, "便携模式（程序目录）")
	if len(lines) < 4 {
		t.Fatalf("关于文案只有 %d 行，期望至少 4 行：%v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "0.1.0-dev") {
		t.Fatalf("首行应含版本号，得到 %q", lines[0])
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"不记录", "不联网", `C:\Users\me\AppData\Local\keyboard-statistics`, "存放方式：便携模式（程序目录）"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("关于文案缺少 %q，实际为：\n%s", want, joined)
		}
	}
}
