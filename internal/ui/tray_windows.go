//go:build windows

package ui

import (
	"log"
	"syscall"
	"unsafe"

	"keyboard-statistics/internal/config"
	"keyboard-statistics/internal/tray"
)

// notifyIconData 对应 Win32 的 NOTIFYICONDATAW（V3 布局）。
//
// 字段顺序和类型必须与头文件逐字对应：Shell_NotifyIconW 会拿 cbSize 校验结构，
// 多了少了都会静默失败（只返回 FALSE，不告诉你哪里错了）。
type notifyIconData struct {
	cbSize           uint32
	hWnd             syscall.Handle
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            syscall.Handle
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uTimeout         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     syscall.Handle
}

// putUTF16 把 Go 字符串写进固定长度的宽字符数组，超出部分截断（保证结尾有 0）。
func putUTF16(dst []uint16, s string) {
	if len(dst) == 0 {
		return
	}
	src := syscall.StringToUTF16(s)
	n := copy(dst[:len(dst)-1], src)
	// StringToUTF16 的末尾带一个 0，copy 可能没拷到，显式补上
	if n < len(dst) {
		dst[n] = 0
	}
}

// loadAppIcon 取托盘图标。
//
// 优先用 exe 自己的图标（go-winres 把 assets/app.ico 打进资源，ID 是 1），
// 拿不到就退回系统默认图标——宁可图标难看，也不要不显示托盘图标。
func loadAppIcon() syscall.Handle {
	inst, _, _ := procGetModuleHandleW.Call(0)
	if inst != 0 {
		h, _, _ := procLoadImageW.Call(
			inst, 1, imageIcon, 0, 0, lrShared|lrDefaultSize)
		if h != 0 {
			return syscall.Handle(h)
		}
		h, _, _ = procLoadIconW.Call(inst, 1)
		if h != 0 {
			return syscall.Handle(h)
		}
	}
	h, _, _ := procLoadIconW.Call(0, idiApplication)
	return syscall.Handle(h)
}

// trayInit 在托盘区放上图标。失败只是没有托盘图标，不影响程序继续跑。
func (a *App) trayInit() {
	a.trayIcon = loadAppIcon()

	nid := a.notifyIcon(nifMessage|nifIcon|nifTip, a.svc.Collecting())
	r, _, _ := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
	a.trayAdded = r != 0
}

// notifyIcon 拼一个通知图标结构。tip 每次重建，避免调用方忘了同步提示文字。
func (a *App) notifyIcon(flags uint32, collecting bool) notifyIconData {
	nid := notifyIconData{
		cbSize:           uint32(unsafe.Sizeof(notifyIconData{})),
		hWnd:             a.hwnd,
		uID:              trayIconID,
		uFlags:           flags,
		uCallbackMessage: msgTray,
		hIcon:            a.trayIcon,
	}
	putUTF16(nid.szTip[:], tray.Tooltip(collecting))
	return nid
}

// trayRemove 摘掉托盘图标。程序退出前必须调用，否则图标会残留在托盘里直到鼠标划过。
func (a *App) trayRemove() {
	if !a.trayAdded {
		return
	}
	nid := a.notifyIcon(0, true)
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
	a.trayAdded = false
}

// traySetTip 在采集状态变化后刷新悬停提示。
func (a *App) traySetTip(collecting bool) {
	if !a.trayAdded {
		return
	}
	nid := a.notifyIcon(nifTip, collecting)
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

// trayBalloon 弹一个气泡提示。只在第一次隐藏到托盘时用。
func (a *App) trayBalloon(title, text string) {
	if !a.trayAdded {
		return
	}
	nid := a.notifyIcon(nifInfo, a.svc.Collecting())
	putUTF16(nid.szInfoTitle[:], title)
	putUTF16(nid.szInfo[:], text)
	nid.dwInfoFlags = niifInfo
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

// trayPopupMenu 弹出右键菜单并执行选中的动作。
func (a *App) trayPopupMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	for _, it := range tray.Menu(a.svc.Collecting(), a.closeAction.ToTray()) {
		if it.Separator {
			procAppendMenuW.Call(menu, mfSeparator, 0, 0)
			continue
		}
		flags := uintptr(mfString)
		if it.Checked {
			flags |= mfChecked
		}
		procAppendMenuW.Call(menu, flags, uintptr(it.ID), utf16Ptr(it.Label))
	}

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	// 必须先把自己的窗口设为前台，否则点菜单外面时菜单不会消失（Win32 经典坑）。
	procSetForegroundWin.Call(uintptr(a.hwnd))
	cmd, _, _ := procTrackPopupMenu.Call(
		menu, tpmRightButton|tpmReturnCmd, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(a.hwnd), 0)
	// 再补一条空消息，让菜单真正走完它自己的消息循环。
	postMessage(a.hwnd, wmNull, 0, 0)

	action, ok := tray.ActionFor(uint32(cmd))
	if !ok {
		return
	}
	switch action {
	case tray.ActionShow:
		a.showFromTray()
	case tray.ActionToggleCollect:
		a.toggleCollecting()
	case tray.ActionToggleCloseToTray:
		a.toggleCloseToTray()
	case tray.ActionExit:
		a.quit()
	}
}

// toggleCloseToTray 在托盘菜单里切换关窗行为。
//
// 关窗时那个选项框只会问一次，之后想改就得有个入口——托盘菜单是最顺手的地方，
// 不用为了一个开关去翻设置页。
func (a *App) toggleCloseToTray() {
	a.setCloseAction(!a.closeAction.ToTray())
}

// askCloseAction 在用户第一次点关闭按钮时问一次该怎么做。
//
// 用户按了 X 却什么都不发生（也没关、也没提示）是最糟的体验，所以取消也要有反馈。
func (a *App) askCloseAction() {
	title, text := tray.CloseChoice()
	switch messageBox(a.hwnd, text, title, mbYesNoCancel|mbIconQuestion) {
	case idYes:
		a.setCloseAction(true)
		a.hideToTray()
	case idNo:
		a.setCloseAction(false)
		a.quit()
	default:
		// 取消：窗口就留着，什么都不做
	}
}

// setCloseAction 记住「关窗时是否收进托盘」，并同步托盘菜单的勾选状态。
//
// 存不下来（没有设置存储/注册表不可写）不阻断操作：这次的选择立即生效，
// 只是下次启动会再问一遍——总比弹个错误框阻断关窗要好。
func (a *App) setCloseAction(toTray bool) {
	action := config.CloseExit
	if toTray {
		action = config.CloseToTray
	}
	a.closeAction = action
	// 写不进去也不阻断：这次的选择已生效，代价只是下次启动会再问一遍。
	// 界面层没有日志器，`-H windowsgui` 下这行也看不到——保留它是为了让
	// `go run ./cmd/app` 调试时能发现。
	if err := config.SetCloseAction(a.opt.SettingsStore, action); err != nil {
		log.Printf("无法保存关窗行为（下次会再问一遍）: %v", err)
	}
}

// onTrayMessage 处理托盘图标的鼠标消息。
// 没调用 NIM_SETVERSION，所以 lParam 直接就是鼠标消息（老版本行为）。
func (a *App) onTrayMessage(lParam uintptr) {
	switch uint32(lParam) {
	case wmLButtonUp, wmLButtonDblClk:
		a.showFromTray()
	case wmRButtonUp:
		a.trayPopupMenu()
	}
}

// hideToTray 把窗口收进托盘，但**不退出**：采集继续，进程常驻。
func (a *App) hideToTray() {
	// 先撤掉一切模态状态（尤其是正在弹出的托盘菜单）：
	// 菜单是端着消息循环等的，窗口藏起来之后它就再没人点得到，会一直挂在那里。
	procSendMessageW.Call(uintptr(a.hwnd), uintptr(wmCancelMode), 0, 0)
	showWindow(a.hwnd, swHide)

	// 用户按 X 却发现程序没退出，必须立刻解释一次，否则会以为关不掉。
	// 只弹「第一次」——这个标记存在设置里，跨启动也算数，不然每次开机都被弹一遍。
	if !a.trayHintShown {
		a.trayHintShown = true
		if err := config.SetFlag(a.opt.SettingsStore, config.FlagTrayHintShown, true); err != nil {
			// 存不下就退化成「每次启动弹一次」，比彻底不提示好
			log.Printf("无法记住气泡提示已弹过: %v", err)
		}
		title, text := tray.HiddenHint()
		a.trayBalloon(title, text)
	}
}

// showFromTray 把窗口从托盘里放出来并激活。
func (a *App) showFromTray() {
	showExistingWindow(a.hwnd)
	invalidate(a.hwnd)

	// 开机自启时欠下的欢迎说明：窗口第一次露面时补上。
	if a.onboardPending {
		a.onboardPending = false
		a.maybeOnboard()
	}
}

// showExistingWindow 让一个可能被隐藏或最小化的窗口重新出现在前台。
//
// 顺序有讲究：隐藏的窗口（收进托盘）用 SW_RESTORE 不一定能显示出来，
// 所以先按可见性补一个 SW_SHOW，再 SW_RESTORE 处理「最小化了」的情况。
func showExistingWindow(h syscall.Handle) {
	if visible, _, _ := procIsWindowVisible.Call(uintptr(h)); visible == 0 {
		showWindow(h, swShow)
	}
	showWindow(h, swRestore)
	procSetForegroundWin.Call(uintptr(h))
}

// registerQuitMessage 注册「升级前请退出」这条消息，返回消息号（失败为 0）。
//
// 必须和安装器注册同一个字符串，两边才会拿到同一个消息号。
func registerQuitMessage() uint32 {
	name := syscall.StringToUTF16Ptr(tray.QuitMessageName)
	r, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(name)))
	return uint32(r)
}

// quit 真正退出：销毁窗口，消息循环随之结束。
//
// 注意不能再用 postMessage(WM_CLOSE)：WM_CLOSE 现在的语义是「收进托盘」，
// 用它退出会让程序以为只是最小化了，改数据目录那种需要重启的流程会卡住。
func (a *App) quit() {
	destroyWindow(a.hwnd)
}
