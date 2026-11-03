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
	// ActionToggleCloseToTray 切换「关闭窗口时是收进托盘还是直接退出」。
	ActionToggleCloseToTray
	// ActionExit 正常退出（会把没落库的计数刷进去）。
	ActionExit
)

// 菜单项 ID。窗口过程靠它们分发命令，改值等于改协议。
const (
	IDShow        uint32 = 1
	IDToggle      uint32 = 2
	IDExit        uint32 = 3
	IDCloseToTray uint32 = 4
)

// Item 是一个托盘菜单项。
type Item struct {
	ID        uint32
	Label     string
	Separator bool // 为 true 时画成分隔线，ID/Label 无意义
	Checked   bool // 显示勾选标记，用来表示「当前处于这个状态」
}

// Menu 按当前采集状态与关窗行为给出右键菜单。
//
// 两处状态都要反映到勾选上：采集中的第 3 项是「暂停采集」，暂停时变成「继续采集」并打勾；
// 「关闭窗口时收进托盘」按实际设置打勾。托盘菜单最怕的就是看了不知道现在是什么状态。
func Menu(collecting, closeToTray bool) []Item {
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
		{ID: IDCloseToTray, Label: "关闭窗口时收进托盘", Checked: closeToTray},
		{Separator: true},
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
	case IDCloseToTray:
		return ActionToggleCloseToTray, true
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

// QuitMessageName 是「请退出，我要升级了」这条注册消息的名字。
//
// 安装器先发这条消息、等进程自己退出，再替换文件。为什么不用 WM_CLOSE 代替：
// 托盘常驻之后关闭窗口只是「收进托盘」，程序不会退；Restart Manager 只能干等
// 三十秒再强杀，那段时间里内存中还没落库的计数会丢。
// 两个进程各自 RegisterWindowMessage 同一个名字会拿到同一个消息号，这就是握手的基础。
const QuitMessageName = "KeyboardStatistics.QuitForUpgrade"

// HiddenHint 是第一次隐藏到托盘时的气泡提示文案。
//
// 用户按了 X 却发现程序没退出，必须马上解释一次，否则会以为关不掉。
func HiddenHint() (title, text string) {
	return "键盘使用统计仍在后台运行",
		fmt.Sprintf("窗口已收到托盘，统计继续进行。\n右键托盘图标可以暂停采集或退出。")
}

// AutostartHint 是开机自启时的一次性提示。
//
// 注意：当前**没有**在自启时弹气泡（用户明确要求开机自启不要任何提示），
// 这个文案留着是为了以后需要时不用重新设计，测试也仍然盯着它与 HiddenHint 的区别。
func AutostartHint() (title, text string) {
	return "键盘使用统计已在后台开始统计",
		"点托盘图标可以查看统计；右键可以暂停采集或退出。"
}

// CloseChoice 是用户第一次点关闭按钮时弹的选项框文案。
//
// 系统按钮只有「是 / 否 / 取消」三个词，所以正文必须逐条说明各自代表什么，
// 否则用户只能猜；结尾还要告诉他以后去哪儿改。
func CloseChoice() (title, text string) {
	return "关闭窗口后要怎么做？",
		"是：收进托盘，统计继续进行\n" +
			"否：退出程序，停止统计\n" +
			"取消：先不关\n\n" +
			"这个选择会记住；以后可以在托盘右键菜单里改。"
}
