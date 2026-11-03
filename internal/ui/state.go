package ui

import (
	"fmt"
	"strings"
)

// 本文件只放「不碰 Win32 的界面判定与文案」：按钮该不该动注册表、状态该显示什么、
// 关于区块写哪几行。它们没有窗口句柄依赖，所以能用表驱动测试锁住行为。

// autostartOp 是点击开机自启复选框后要执行的注册表动作。
type autostartOp int

const (
	autostartNoop autostartOp = iota
	autostartEnable
	autostartDisable
)

// autostartAction 决定复选框这次「变化」要不要真的写注册表。
//
// updating 为真说明这次勾选状态是程序自己刷新上去的（每 2 秒同步一次自启状态），
// 此时必须什么都不做：否则会把「读到的状态」再写回注册表，读失败时还会来回开关。
func autostartAction(updating, checked bool) autostartOp {
	if updating {
		return autostartNoop
	}
	if checked {
		return autostartEnable
	}
	return autostartDisable
}

// collectButtonText 是采集开关按钮的文字：写「暂停」时它正在采集。
func collectButtonText(collecting bool) string {
	if collecting {
		return "暂停采集"
	}
	return "继续采集"
}

// statusText 是顶部右侧的采集状态。
func statusText(collecting bool) string {
	if collecting {
		return "● 采集中"
	}
	return "● 已暂停"
}

// staleWarningText 在自启路径指向别的 exe 时给出提示；未失效时返回空串。
//
// 只提示不修复：用户移动过 exe 之后该不该重新写注册表，得由他自己决定。
func staleWarningText(stale bool) string {
	if !stale {
		return ""
	}
	return "⚠ 自启路径已失效，请取消后重新勾选"
}

// rangeSummary 是底部状态栏里的一句话摘要。
func rangeSummary(from, to string, days int) string {
	s := fmt.Sprintf("%s ~ %s", from, to)
	if days > 0 {
		s += fmt.Sprintf(" · 共 %d 天", days)
	}
	return s
}

// aboutLines 是「关于」区块的文案。
//
// 这几行同时承担隐私说明的职责（只记什么、不记什么、数据在哪、不联网），
// 改文案时不要删掉任何一类信息。
// source 非空时额外说明数据目录是怎么定的，用户改过目录后能一眼看清。
func aboutLines(version, dir, source string) []string {
	lines := []string{
		"键盘使用统计 v" + version,
		"只记录键码、次数、日期与小时，不记录任何输入内容。",
		"不记录按键顺序、窗口标题、应用名；不联网，数据只在本机。",
		"数据目录：" + dir,
	}
	if source != "" {
		lines = append(lines, "存放方式："+source)
	}
	return lines
}

// onboardingText 是首次启动说明框的文案。
//
// 采集默认就是开的，用户第一眼必须知道「已经在采、采什么、不采什么、怎么停」。
// 这是红线 3「不隐藏采集」的落点，三句话缺一不可（有测试盯着）。
func onboardingText() string {
	return "键盘使用统计已经开始采集了，不需要你再点任何按钮。\n\n" +
		"• 只记录每个键按了多少次、用过哪些快捷键、每天活跃多久\n" +
		"• 不记录你输入的任何内容，不记录按键顺序，也不记录窗口标题或应用名\n" +
		"• 数据只保存在本机，不联网上传\n\n" +
		"随时可以暂停：设置页有「暂停采集」，窗口右上角会一直显示「● 采集中」或「● 已暂停」。\n\n" +
		"这个说明只弹这一次，之后不再打扰。"
}

// joinNonEmpty 过滤空串，用于把可选提示拼进绘制流程。
func joinNonEmpty(parts ...string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}
