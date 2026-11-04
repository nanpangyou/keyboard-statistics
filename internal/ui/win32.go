//go:build windows

package ui

import (
	"fmt"
	"syscall"
	"unsafe"
)

// 本文件是界面层唯一碰 Win32 的地方：只加载 Windows 自带 DLL，
// 不依赖 OpenGL / DirectX，也不需要 cgo（云桌面与远程桌面都能用）。
var (
	dllUser32   = syscall.NewLazyDLL("user32.dll")
	dllGdi32    = syscall.NewLazyDLL("gdi32.dll")
	dllKernel32 = syscall.NewLazyDLL("kernel32.dll")
	dllShell32  = syscall.NewLazyDLL("shell32.dll")
	dllOle32    = syscall.NewLazyDLL("ole32.dll")
)

var (
	procRegisterClassExW         = dllUser32.NewProc("RegisterClassExW")
	procCreateWindowExW          = dllUser32.NewProc("CreateWindowExW")
	procDefWindowProcW           = dllUser32.NewProc("DefWindowProcW")
	procShowWindow               = dllUser32.NewProc("ShowWindow")
	procFindWindowW              = dllUser32.NewProc("FindWindowW")
	procSetForegroundWin         = dllUser32.NewProc("SetForegroundWindow")
	procUpdateWindow             = dllUser32.NewProc("UpdateWindow")
	procGetMessageW              = dllUser32.NewProc("GetMessageW")
	procGetCurrentThreadID       = dllKernel32.NewProc("GetCurrentThreadId")
	procGetWindowThreadProcessID = dllUser32.NewProc("GetWindowThreadProcessId")
	procTranslateMessage         = dllUser32.NewProc("TranslateMessage")
	procDispatchMessageW         = dllUser32.NewProc("DispatchMessageW")
	procPostQuitMessage          = dllUser32.NewProc("PostQuitMessage")
	procDestroyWindow            = dllUser32.NewProc("DestroyWindow")
	procSetTimer                 = dllUser32.NewProc("SetTimer")
	procKillTimer                = dllUser32.NewProc("KillTimer")
	procInvalidateRect           = dllUser32.NewProc("InvalidateRect")
	procBeginPaint               = dllUser32.NewProc("BeginPaint")
	procEndPaint                 = dllUser32.NewProc("EndPaint")
	procLoadCursorW              = dllUser32.NewProc("LoadCursorW")
	procGetSystemMetrics         = dllUser32.NewProc("GetSystemMetrics")
	procSetWindowPos             = dllUser32.NewProc("SetWindowPos")
	procGetWindowRect            = dllUser32.NewProc("GetWindowRect")
	procGetClientRect            = dllUser32.NewProc("GetClientRect")
	procDrawTextW                = dllUser32.NewProc("DrawTextW")
	procFillRect                 = dllUser32.NewProc("FillRect")
	procMessageBoxW              = dllUser32.NewProc("MessageBoxW")
	procPostMessageW             = dllUser32.NewProc("PostMessageW")
	procSendMessageW             = dllUser32.NewProc("SendMessageW")
	procSendMessageTimeout       = dllUser32.NewProc("SendMessageTimeoutW")
	procTrackMouseEvent          = dllUser32.NewProc("TrackMouseEvent")
	procSetProcessDPIAware       = dllUser32.NewProc("SetProcessDPIAware")
	procGetDpiForWindow          = dllUser32.NewProc("GetDpiForWindow")
	procGetDC                    = dllUser32.NewProc("GetDC")
	procReleaseDC                = dllUser32.NewProc("ReleaseDC")

	procCreateCompatibleDC     = dllGdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = dllGdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = dllGdi32.NewProc("SelectObject")
	procDeleteObject           = dllGdi32.NewProc("DeleteObject")
	procDeleteDC               = dllGdi32.NewProc("DeleteDC")
	procBitBlt                 = dllGdi32.NewProc("BitBlt")
	procCreateSolidBrush       = dllGdi32.NewProc("CreateSolidBrush")
	procCreateFontW            = dllGdi32.NewProc("CreateFontW")
	procSetBkMode              = dllGdi32.NewProc("SetBkMode")
	procSetTextColor           = dllGdi32.NewProc("SetTextColor")
	procGetDeviceCaps          = dllGdi32.NewProc("GetDeviceCaps")

	procGetModuleHandleW = dllKernel32.NewProc("GetModuleHandleW")

	procSHBrowseForFolderW   = dllShell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDListW = dllShell32.NewProc("SHGetPathFromIDListW")
	procShellExecuteW        = dllShell32.NewProc("ShellExecuteW")
	procShellNotifyIconW     = dllShell32.NewProc("Shell_NotifyIconW")

	// 托盘相关：弹出菜单与图标
	procCreatePopupMenu = dllUser32.NewProc("CreatePopupMenu")
	procAppendMenuW     = dllUser32.NewProc("AppendMenuW")
	procTrackPopupMenu  = dllUser32.NewProc("TrackPopupMenu")
	procDestroyMenu     = dllUser32.NewProc("DestroyMenu")
	procGetCursorPos    = dllUser32.NewProc("GetCursorPos")
	procLoadImageW      = dllUser32.NewProc("LoadImageW")
	procLoadIconW       = dllUser32.NewProc("LoadIconW")
	procIsWindowVisible = dllUser32.NewProc("IsWindowVisible")

	procRegisterWindowMessageW = dllUser32.NewProc("RegisterWindowMessageW")

	procCoTaskMemFree = dllOle32.NewProc("CoTaskMemFree")
)

// 窗口、消息、GDI 常量。名字都保留 Win32 原名的意思，方便对着 MSDN 查。
const (
	csHRedraw = 0x0002
	csVRedraw = 0x0001

	wsOverlappedWindow = 0x00CF0000

	swShowNormal        = 1
	swShowMinNoActivate = 7
	swShow              = 5
	swRestore           = 9
	swHide              = 0

	swpNoSize   = 0x0001
	swpNoMove   = 0x0002
	swpNoZOrder = 0x0004

	wmDestroy       = 0x0002
	wmSize          = 0x0005
	wmPaint         = 0x000F
	wmClose         = 0x0010
	wmEraseBkgnd    = 0x0014
	wmGetMinMaxInfo = 0x0024
	wmTimer         = 0x0113
	wmMouseMove     = 0x0200
	wmLButtonDown   = 0x0201
	wmMouseLeave    = 0x02A3
	wmApp           = 0x8000

	// 托盘
	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205
	wmNull          = 0x0000
	wmSetIcon       = 0x0080
	wmCancelMode    = 0x001F

	nimAdd    = 0
	nimModify = 1
	nimDelete = 2

	nifMessage = 0x0001
	nifIcon    = 0x0002
	nifTip     = 0x0004
	nifInfo    = 0x0010

	niifInfo = 0x0001

	mfString    = 0x0000
	mfChecked   = 0x0008
	mfSeparator = 0x0800

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100

	smtoBlock       = 0x0001
	smtoAbortIfHung = 0x0002

	imageIcon      = 1
	lrShared       = 0x8000
	lrDefaultSize  = 0x0040
	idiApplication = 32512

	// 窗口图标：ICON_SMALL 给标题栏，ICON_BIG 给 Alt+Tab 和任务栏。
	iconSmall = 0
	iconBig   = 1

	// 托盘图标的回调消息与图标 ID
	msgTray    = wmApp + 2
	trayIconID = 1

	idcArrow = 32512

	smCxScreen = 0
	smCyScreen = 1

	logPixelsY = 90

	tmeLeave = 0x0002

	dtCenter      = 0x0001
	dtRight       = 0x0002
	dtVCenter     = 0x0004
	dtSingleLine  = 0x0020
	dtWordBreak   = 0x0010
	dtEndEllipsis = 0x8000
	dtNoPrefix    = 0x0800

	transparentBk  = 1
	cleartypeQP    = 5
	defaultCharset = 1

	fwNormal   = 400
	fwSemiBold = 600
	fwBold     = 700

	srcCopy = 0x00CC0020

	mbOK           = 0x0000
	mbYesNo        = 0x0004
	mbYesNoCancel  = 0x0003
	mbIconError    = 0x0010
	mbIconQuestion = 0x0020
	mbIconWarning  = 0x0030
	mbIconInfo     = 0x0040
	mbDefButton1   = 0x0000
	mbDefButton2   = 0x0100
	idYes          = 6
	idNo           = 7
	idCancel       = 2

	bifReturnOnlyFSDirs = 0x0001
	maxPath             = 260
)

// wndClassEx 对应 WNDCLASSEXW。
type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   syscall.Handle
	Icon       syscall.Handle
	Cursor     syscall.Handle
	Background syscall.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     syscall.Handle
}

type point struct{ X, Y int32 }

// msg 对应 MSG。
type msg struct {
	Hwnd    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

// paintStruct 对应 PAINTSTRUCT。
type paintStruct struct {
	Hdc       syscall.Handle
	Erase     int32
	Paint     Rect
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}

// minMaxInfo 对应 MINMAXINFO，用于限制窗口最小尺寸。
type minMaxInfo struct {
	Reserved     point
	MaxSize      point
	MaxPosition  point
	MinTrackSize point
	MaxTrackSize point
}

type trackMouseEvent struct {
	Size      uint32
	Flags     uint32
	Track     syscall.Handle
	HoverTime uint32
}

// browseInfoW 对应 BROWSEINFOW，选目录对话框要用。
type browseInfoW struct {
	Owner        syscall.Handle
	Root         uintptr
	DisplayName  *uint16
	Title        *uint16
	Flags        uint32
	Callback     uintptr
	CallbackData uintptr
	Image        int32
}

// utf16Str 把 Go 字符串转成 Win32 需要的 UTF-16 指针。
//
// 结构体字段一律用它，而不是先转 uintptr 再转回指针：那种写法 go vet 会告警，
// 而且中间隔了一层 uintptr 后指针确实可能被 GC 提前回收。
func utf16Str(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}
	return p
}

// utf16Ptr 与 utf16Str 相同，但用于直接传给 syscall.Call 的参数位。
func utf16Ptr(s string) uintptr {
	p := utf16Str(s)
	if p == nil {
		return 0
	}
	return uintptr(unsafe.Pointer(p))
}

// registerClass 注册窗口类；同名类重复注册会失败，所以只调用一次。
func registerClass(name string, wndProc uintptr) (syscall.Handle, error) {
	hInst, _, _ := procGetModuleHandleW.Call(0)
	if hInst == 0 {
		return 0, fmt.Errorf("GetModuleHandleW 失败")
	}
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)

	wc := wndClassEx{
		Style:     csHRedraw | csVRedraw,
		WndProc:   wndProc,
		Instance:  syscall.Handle(hInst),
		Cursor:    syscall.Handle(cursor),
		ClassName: utf16Str(name),
	}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return 0, fmt.Errorf("RegisterClassExW(%s): %w", name, err)
	}
	return syscall.Handle(hInst), nil
}

func createWindow(className, title string, w, h int32, hInst syscall.Handle) (syscall.Handle, error) {
	r, _, err := procCreateWindowExW.Call(
		0,
		utf16Ptr(className),
		utf16Ptr(title),
		wsOverlappedWindow,
		0, 0, uintptr(w), uintptr(h),
		0, 0, uintptr(hInst), 0,
	)
	if r == 0 {
		return 0, fmt.Errorf("CreateWindowExW: %w", err)
	}
	return syscall.Handle(r), nil
}

// loadWindowIcon 按指定像素边长取 exe 内嵌的应用图标（资源 id 1，见 assets\winres.json）。
//
// 和托盘那份的区别是这里显式要尺寸：标题栏要 16px、Alt+Tab 要 32px，
// 让系统在 9 档尺寸里挑最合适的一档，比给它一张大图临时缩要清楚得多。
// 取不到返回 0，调用方就别动图标（保持系统默认，总比空白好）。
func loadWindowIcon(size int32) syscall.Handle {
	inst, _, _ := procGetModuleHandleW.Call(0)
	if inst == 0 || size <= 0 {
		return 0
	}
	h, _, _ := procLoadImageW.Call(inst, 1, imageIcon, uintptr(size), uintptr(size), lrShared)
	if h == 0 {
		return 0
	}
	return syscall.Handle(h)
}

// setWindowIcon 把图标挂到窗口上。
//
// 只给窗口类设图标是不够的：任务栏、Alt+Tab、标题栏各取各的（ICON_BIG / ICON_SMALL），
// 而且类图标在建窗口时若还没备好就再也补不上，所以这里两只都显式换一遍。
// 不换的话窗口左上角是系统默认的白板图标——用户一眼就看出「这不是正式版」。
func setWindowIcon(h syscall.Handle, small, big syscall.Handle) {
	if h == 0 {
		return
	}
	if small != 0 {
		procSendMessageW.Call(uintptr(h), wmSetIcon, iconSmall, uintptr(small))
	}
	if big != 0 {
		procSendMessageW.Call(uintptr(h), wmSetIcon, iconBig, uintptr(big))
	}
}

func defWindowProc(h syscall.Handle, message uint32, wParam, lParam uintptr) uintptr {
	r, _, _ := procDefWindowProcW.Call(uintptr(h), uintptr(message), wParam, lParam)
	return r
}

func showWindow(h syscall.Handle, cmd int32) {
	procShowWindow.Call(uintptr(h), uintptr(cmd))
}

func updateWindow(h syscall.Handle) { procUpdateWindow.Call(uintptr(h)) }

func destroyWindow(h syscall.Handle) { procDestroyWindow.Call(uintptr(h)) }

func postQuitMessage(code int32) { procPostQuitMessage.Call(uintptr(code)) }

func invalidate(h syscall.Handle) { procInvalidateRect.Call(uintptr(h), 0, 0) }

func postMessage(h syscall.Handle, message uint32, wParam, lParam uintptr) {
	procPostMessageW.Call(uintptr(h), uintptr(message), wParam, lParam)
}

// sendMessageTimeout 给窗口发一条消息并等它处理完，超时就放弃。
//
// 返回值是「窗口线程有没有在超时前处理掉这条消息」：拿它当界面线程的心跳探针，
// 比去猜「CPU 是 0 还是 100%」靠谱得多。SMTO_ABORTIFHUNG 让它在对面已经卡住时
// 立刻返回，不必等满超时（线上诊断要的就是尽快拿到现场）。
func sendMessageTimeout(h syscall.Handle, message uint32, timeoutMs uint32) bool {
	var result uintptr
	r, _, _ := procSendMessageTimeout.Call(
		uintptr(h), uintptr(message), 0, 0,
		smtoBlock|smtoAbortIfHung, uintptr(timeoutMs), uintptr(unsafe.Pointer(&result)))
	return r != 0
}

// getMessage 返回 0 表示收到 WM_QUIT，-1 表示出错。
func getMessage(m *msg) int32 {
	r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(m)), 0, 0, 0)
	return int32(r)
}

// getCurrentThreadID 返回当前 OS 线程的 id（消息队列就挂在它上面）。
func getCurrentThreadID() uint32 {
	r, _, _ := procGetCurrentThreadID.Call()
	return uint32(r)
}

// windowThreadID 返回某个窗口所属的线程 id；窗口不存在时是 0。
func windowThreadID(h syscall.Handle) uint32 {
	var pid uint32
	r, _, _ := procGetWindowThreadProcessID.Call(uintptr(h), uintptr(unsafe.Pointer(&pid)))
	return uint32(r)
}

func translateMessage(m *msg) { procTranslateMessage.Call(uintptr(unsafe.Pointer(m))) }

func dispatchMessage(m *msg) { procDispatchMessageW.Call(uintptr(unsafe.Pointer(m))) }

func setTimer(h syscall.Handle, id uintptr, ms uint32) bool {
	r, _, _ := procSetTimer.Call(uintptr(h), id, uintptr(ms), 0)
	return r != 0
}

func killTimer(h syscall.Handle, id uintptr) { procKillTimer.Call(uintptr(h), id) }

func clientRect(h syscall.Handle) Rect {
	var r Rect
	procGetClientRect.Call(uintptr(h), uintptr(unsafe.Pointer(&r)))
	return r
}

func windowRect(h syscall.Handle) Rect {
	var r Rect
	procGetWindowRect.Call(uintptr(h), uintptr(unsafe.Pointer(&r)))
	return r
}

func setWindowPos(h syscall.Handle, x, y, w, hgt int32, flags uint32) {
	procSetWindowPos.Call(uintptr(h), 0, uintptr(x), uintptr(y), uintptr(w), uintptr(hgt), uintptr(flags))
}

func screenSize() (int32, int32) {
	w, _, _ := procGetSystemMetrics.Call(smCxScreen)
	h, _, _ := procGetSystemMetrics.Call(smCyScreen)
	return int32(w), int32(h)
}

func beginPaint(h syscall.Handle, ps *paintStruct) syscall.Handle {
	r, _, _ := procBeginPaint.Call(uintptr(h), uintptr(unsafe.Pointer(ps)))
	return syscall.Handle(r)
}

func endPaint(h syscall.Handle, ps *paintStruct) {
	procEndPaint.Call(uintptr(h), uintptr(unsafe.Pointer(ps)))
}

func fillRect(hdc syscall.Handle, r Rect, brush syscall.Handle) {
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&r)), uintptr(brush))
}

func drawText(hdc syscall.Handle, s string, r Rect, flags uint32) {
	if s == "" {
		return
	}
	p := utf16Ptr(s)
	if p == 0 {
		return
	}
	procDrawTextW.Call(uintptr(hdc), p, ^uintptr(0), uintptr(unsafe.Pointer(&r)), uintptr(flags))
}

func createSolidBrush(color uint32) syscall.Handle {
	r, _, _ := procCreateSolidBrush.Call(uintptr(color))
	return syscall.Handle(r)
}

func deleteObject(h syscall.Handle) { procDeleteObject.Call(uintptr(h)) }

func createFont(height int32, weight int32, face string) syscall.Handle {
	r, _, _ := procCreateFontW.Call(
		uintptr(height), 0, 0, 0, uintptr(weight),
		0, 0, 0,
		defaultCharset, 0, 0, cleartypeQP, 0,
		utf16Ptr(face),
	)
	return syscall.Handle(r)
}

func selectObject(hdc, obj syscall.Handle) syscall.Handle {
	r, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(obj))
	return syscall.Handle(r)
}

func setBkMode(hdc syscall.Handle, mode int32) {
	procSetBkMode.Call(uintptr(hdc), uintptr(mode))
}

func setTextColor(hdc syscall.Handle, color uint32) {
	procSetTextColor.Call(uintptr(hdc), uintptr(color))
}

func createCompatibleDC(hdc syscall.Handle) syscall.Handle {
	r, _, _ := procCreateCompatibleDC.Call(uintptr(hdc))
	return syscall.Handle(r)
}

func createCompatibleBitmap(hdc syscall.Handle, w, h int32) syscall.Handle {
	r, _, _ := procCreateCompatibleBitmap.Call(uintptr(hdc), uintptr(w), uintptr(h))
	return syscall.Handle(r)
}

func deleteDC(hdc syscall.Handle) { procDeleteDC.Call(uintptr(hdc)) }

func bitBlt(dst syscall.Handle, w, h int32, src syscall.Handle) bool {
	r, _, _ := procBitBlt.Call(uintptr(dst), 0, 0, uintptr(w), uintptr(h), uintptr(src), 0, 0, srcCopy)
	return r != 0
}

// setProcessDPIAware 让坐标直接用物理像素，否则高分屏会被系统整体拉伸变糊。
func setProcessDPIAware() { procSetProcessDPIAware.Call() }

// dpiForWindow 拿窗口所在显示器的 DPI；老系统没有 GetDpiForWindow 时退回设备能力查询。
func dpiForWindow(h syscall.Handle) int32 {
	if err := procGetDpiForWindow.Find(); err == nil {
		if r, _, _ := procGetDpiForWindow.Call(uintptr(h)); r != 0 {
			return int32(r)
		}
	}
	if hdc, _, _ := procGetDC.Call(uintptr(h)); hdc != 0 {
		defer procReleaseDC.Call(uintptr(h), hdc)
		if r, _, _ := procGetDeviceCaps.Call(hdc, logPixelsY); r != 0 {
			return int32(r)
		}
	}
	return 96
}

// trackMouseLeave 让窗口在鼠标离开后能收到 WM_MOUSELEAVE，用来清掉悬停高亮。
func trackMouseLeave(h syscall.Handle) {
	tme := trackMouseEvent{Flags: tmeLeave, Track: h}
	tme.Size = uint32(unsafe.Sizeof(tme))
	procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
}

func messageBox(owner syscall.Handle, text, caption string, flags uint32) int32 {
	r, _, _ := procMessageBoxW.Call(uintptr(owner), utf16Ptr(text), utf16Ptr(caption), uintptr(flags))
	return int32(r)
}

// browseForFolder 弹系统选目录对话框；用户取消时 ok 为 false。
//
// 用老式 SHBrowseForFolderW 而不是 IFileDialog：不引 COM 初始化，够用。
func browseForFolder(owner syscall.Handle, title string) (string, bool) {
	display := make([]uint16, maxPath)

	bi := browseInfoW{
		Owner:       owner,
		DisplayName: &display[0],
		Title:       utf16Str(title),
		Flags:       bifReturnOnlyFSDirs,
	}

	pidl, _, _ := procSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return "", false
	}
	defer procCoTaskMemFree.Call(pidl)

	path := make([]uint16, maxPath)
	if r, _, _ := procSHGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(&path[0]))); r == 0 {
		return "", false
	}
	return syscall.UTF16ToString(path), true
}

// shellOpen 用系统默认程序打开文件或目录（这里是「打开数据目录」）。
func shellOpen(path string) error {
	const swShowNormalVal = 1
	r, _, _ := procShellExecuteW.Call(
		0,
		utf16Ptr("open"),
		utf16Ptr(path),
		0, 0,
		swShowNormalVal,
	)
	if r <= 32 {
		return fmt.Errorf("ShellExecuteW(open %s) 返回 %d", path, r)
	}
	return nil
}
