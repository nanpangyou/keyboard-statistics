// Package tray 定义托盘图标的菜单与提示文案。
//
// 这里只有纯逻辑（能写测试的那部分）：菜单项、ID 到动作的映射、悬停提示。
// 真正调 Shell_NotifyIcon 的代码在 internal/ui，那边只能靠手动验证。
package tray

import "fmt"

// Action 是托盘菜单能触发的动作。
type Action int

const (
	// ActionShow 把主窗口显示出来并激活。
	ActionShow Action = iota
	// ActionToggleCollect 暂停或继续采集。
	ActionToggleCollect
	// ActionExit 正常退出（会把没落库的计数刷进去）。
	ActionExit
)

// 菜单项 ID。窗口过程靠它们分发命令，改值等于改协议。
const (
	IDShow   uint32 = 1
	IDToggle uint32 = 2
	IDExit   uint32 = 3
)

// Item 是一个托盘菜单项。
type Item struct {
	ID        uint32
	Label     string
	Separator bool // 为 true 时画成分隔线，ID/Label 无意义
	Checked   bool // 显示勾选标记，用来表示「当前处于这个状态」
}

// Menu 按当前采集状态给出右键菜单。
//
// 采集中的第 3 项是「暂停采集」，暂停时变成「继续采集」并打勾——
// 托盘菜单最怕的就是看了不知道现在是开还是关。
func Menu(collecting bool) []Item {
	toggleLabel := "继续采集"
	checked := true
	if collecting {
		toggleLabel = "暂停采集"
		checked = false
	}
	return []Item{
		{ID: IDShow, Label: "显示主窗口"},
		{Separator: true},
		{ID: IDToggle, Label: toggleLabel, Checked: checked},
		{ID: IDExit, Label: "退出"},
	}
}

// ActionFor 把菜单 ID 映射成动作。ok=false 表示不是我们认识的命令（例如用户点了别处）。
func ActionFor(id uint32) (Action, bool) {
	switch id {
	case IDShow:
		return ActionShow, true
	case IDToggle:
		return ActionToggleCollect, true
	case IDExit:
		return ActionExit, true
	}
	return 0, false
}

// Tooltip 是鼠标悬停在托盘图标上时的提示文字。
//
// 窗口被隐藏到托盘后，这行字是用户唯一能确认「它还在不在统计」的地方，
// 所以必须带上程序名和当前采集状态。
func Tooltip(collecting bool) string {
	if collecting {
		return "键盘使用统计 · 正在采集"
	}
	return "键盘使用统计 · 采集已暂停"
}

// HiddenHint 是第一次隐藏到托盘时的气泡提示文案。
//
// 用户按了 X 却发现程序没退出，必须马上解释一次，否则会以为关不掉。
func HiddenHint() (title, text string) {
	return "键盘使用统计仍在后台运行",
		fmt.Sprintf("窗口已收到托盘，统计继续进行。\n右键托盘图标可以暂停采集或退出。")
}
