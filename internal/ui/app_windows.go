//go:build windows

package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"keyboard-statistics/internal/config"
	"keyboard-statistics/internal/service"
	"keyboard-statistics/internal/stats"
)

const (
	// windowTitle 是窗口标题，自启最小化时靠它在任务栏里被认出来，不要随便改。
	windowTitle = "键盘使用统计"

	// refreshInterval 是界面刷新节奏。
	//
	// 原先 2 秒一次，看着像「统计有延迟」而不像实时：手离开键盘到数字跳动中间
	// 有肉眼可辨的空档。改成 500ms 后基本是「敲一下就跟着动」。
	// 代价是每秒两次 SQLite 聚合 + 一次重绘，实测（便携模式 500 多条真实数据）
	// 单次取数在毫秒级，界面不卡；取数仍然在后台协程里跑，来不及就跳过本次 tick
	// （见 requestRefresh 的 fetching 判断），不会堆积。
	refreshInterval = 500 * time.Millisecond

	clientWidth  = 900
	clientHeight = 680

	minClientWidth  = 800
	minClientHeight = 600

	refreshTimerID = 1

	// msgSnapshotReady 是后台取数完成后送回窗口线程的自定义消息。
	msgSnapshotReady = wmApp + 1
)

// 页面下标与 tabTitles 顺序、pageID 常量三者必须一致。
var tabTitles = []string{"概览", "热力图", "设置"}

type pageID int

const (
	pageOverview pageID = iota
	pageHeatmap
	pageSettings
)

// hoverKind 标识鼠标当前悬停在哪个可点区域。
type hoverKind int

const (
	hoverNone hoverKind = iota
	hoverScope
	hoverTab
	hoverSettings
	hoverHeatmap
)

type hoverTarget struct {
	kind  hoverKind
	index int
}

// Options 是界面初始化参数，字段与旧版保持一致（cmd/app/main.go 不用改）。
type Options struct {
	Version        string
	ExePath        string
	Autostart      config.Registry
	StartMinimized bool

	// DataDir 是当前实际使用的数据目录，DataDirSource 说明它从哪来
	// （便携模式 / 用户指定 / 默认），两者都直接展示在设置页。
	DataDir       string
	DataDirSource string
	// SettingsStore 用来持久化「自定义数据目录」；为 nil 表示当前环境存不了。
	SettingsStore config.SettingStore

	// OnUIStall 在界面线程卡住时被调用（参数是所有 goroutine 的栈）。
	// 界面卡死时进程从外面看和「安静待着」没有区别，只有程序自己还能交出这份现场。
	OnUIStall func(dump string)
}

// settingsState 是设置页里不来自数据库的那部分状态。
type settingsState struct {
	// updating 为真表示当前正在把「注册表里的真实状态」同步到界面上，
	// 此时任何勾选变化都不许回写注册表（见 autostartAction）。
	updating bool

	autostartAvailable bool
	autostartChecked   bool
	autostartStale     bool
}

// snapshotResult 是后台取数协程交给窗口线程的一批数据。
type snapshotResult struct {
	scope      stats.Scope
	snap       service.Snapshot
	err        error
	collecting bool
	at         time.Time
}

// App 持有窗口、界面状态与 service，负责把数据推到界面上。
//
// 会碰 Win32 的字段只在窗口线程读写；后台取数与窗口线程之间只通过 pending 交接。
type App struct {
	opt Options
	svc *service.Service

	hwnd      syscall.Handle
	className string
	scale     float64
	frameW    int32
	frameH    int32
	fonts     map[fontID]syscall.Handle

	// uiThreadID 是跑消息循环的那个 OS 线程，窗口的队列就挂在它上面。
	// 看门狗拿它跟「窗口实际所属的线程」对一下，能一眼看出线程被调度器换掉了——
	// 那会让 GetMessage 等错队列，界面从此不再处理任何消息。
	uiThreadID uint32

	page     pageID
	scope    stats.Scope
	hover    hoverTarget
	settings settingsState

	snap        service.Snapshot
	snapErr     string
	collecting  bool
	lastRefresh time.Time

	mu       sync.Mutex
	pending  *snapshotResult
	fetching bool

	// stopped 在窗口销毁时关闭，用来让看门狗等后台协程自己收工。
	stopped chan struct{}

	// 托盘图标。trayAdded 为假说明没能放上托盘（或已经摘掉），
	// 这时关窗必须真的退出，不能把程序藏起来又没有任何入口。
	trayIcon      syscall.Handle
	trayAdded     bool
	trayHintShown bool
	// onboardPending 为真表示「开机自启起就没露过面，欢迎说明还欠着」。
	onboardPending bool
	// wmQuitRequest 是安装器发来的「请退出，我要替换文件」消息号，0 表示没注册上。
	wmQuitRequest uint32
	// closeAction 是用户选的关窗行为；CloseAsk 表示还没选过，关窗时问一次再记住。
	closeAction config.CloseAction
}

// New 记录参数并返回 App，此时还没有创建窗口。
//
// 窗口留到 Run 里建：创建失败时才能先把错误提示出来，而不是在没有界面的情况下继续跑。
func New(opts Options, svc *service.Service) *App {
	return &App{
		opt:     opts,
		svc:     svc,
		scope:   stats.ScopeToday,
		page:    pageOverview,
		scale:   1,
		stopped: make(chan struct{}),
	}
}

// ShowFatal 把启动阶段的致命错误显示给用户。
//
// 用 -H windowsgui 编译后没有控制台，光 os.Exit(1) 用户看到的就是「双击没反应」。
// 数据目录不可写、数据库损坏这类问题必须弹窗说清楚，否则无从排查。
func ShowFatal(text string) {
	messageBox(0, text, windowTitle, mbOK|mbIconError)
}

// ShowInfo 弹一条普通提示。目前给「程序已经在运行」用：这不是错误，
// 不该用错误图标吓人。
func ShowInfo(text string) {
	messageBox(0, text, windowTitle, mbOK|mbIconInfo)
}

// ActivateExisting 把已经在运行的实例窗口拉到前台。
//
// 返回 false 表示没找到窗口（可能还在建、或者窗口标题变了）。
// 按标题找窗口是有意的：类名带了 pid（同一台机器上多实例测试时不会互相抢注册），
// 所以只能用标题这个稳定标识。
func ActivateExisting() bool {
	title, err := syscall.UTF16PtrFromString(windowTitle)
	if err != nil {
		return false
	}
	h, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
	if h == 0 {
		return false
	}
	// 窗口可能收在托盘里（隐藏）或最小化了，两种情况都要能拉回来。
	showExistingWindow(syscall.Handle(h))
	return true
}

// Run 创建窗口并进入消息循环，直到窗口关闭才返回。
//
// 这里必须把当前协程钉在它现在所在的 OS 线程上，而且要在建窗口之前钉。
// 原因：窗口的消息队列属于「创建它的那个线程」，GetMessage 只从「调用它的那个
// 线程」的队列里取消息。Go 的调度器会把没钉住的协程挪到别的 OS 线程上去，
// 一旦在建窗口后挪走，GetMessage 就等在了新线程的空队列上——窗口再也收不到
// 任何队列消息（定时器不响、快照通知不到、界面不再重绘），而发给窗口的
// SendMessage 又因为原线程没人取消息而超时。从外面看就是「未响应」、
// CPU 0、托盘右键没反应，而且随机发生在启动后几十秒（实测就是这么挂的）。
func (a *App) Run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// DPI 感知必须在建窗口之前设置，否则高分屏上坐标会被系统二次缩放。
	setProcessDPIAware()

	if err := a.createWindow(); err != nil {
		messageBox(0, "界面初始化失败：\n\n"+err.Error(), windowTitle, mbOK|mbIconError)
		return
	}
	defer a.cleanup()

	a.syncSettingsState()
	a.closeAction = config.GetCloseAction(a.opt.SettingsStore)
	a.trayHintShown = config.GetFlag(a.opt.SettingsStore, config.FlagTrayHintShown)
	a.requestRefresh()

	// 记下当前 OS 线程：窗口的队列就在它身上，看门狗用它验证线程没被换掉。
	a.uiThreadID = getCurrentThreadID()

	// 看门狗要在窗口建好之后、进入消息循环之前起来：卡住时它会留下现场。
	go a.watchUIStall()

	// 开机自启拉起时直接待在托盘里：不弹窗口、也不在任务栏占位。
	// 托盘图标没放上就退回原来的「最小化显示」——否则窗口和图标都没有，用户找不到它。
	a.trayInit()
	// 注册升级握手消息：安装器发来这条消息时，程序自己退出好让它替换文件。
	a.wmQuitRequest = registerQuitMessage()
	if a.opt.StartMinimized && a.trayAdded {
		// 开机自启不打扰用户：不弹窗口，也不弹任何气泡提示。
		// 「已经在统计」这件事由托盘图标本身表达，用户主动打开窗口时再补欢迎说明。
		a.onboardPending = true
	} else {
		a.show()
		a.maybeOnboard()
	}

	var m msg
	for {
		r := getMessage(&m)
		if r == 0 || r == -1 {
			return
		}
		translateMessage(&m)
		dispatchMessage(&m)
	}
}

// maybeOnboard 在首次启动时弹一次说明框。
//
// 采集默认开启，所以必须在窗口显示后立刻告知用户，而不是等他去翻设置页。
// 读完就写 meta.onboarded，之后不再打扰；写失败也不阻断启动——最多下次再提示一次。
func (a *App) maybeOnboard() {
	if done, err := a.svc.Onboarded(); err == nil && done {
		return
	}
	messageBox(a.hwnd, onboardingText(), "欢迎使用键盘使用统计", mbOK|mbIconInfo)
	_ = a.svc.MarkOnboarded()
}

func (a *App) createWindow() error {
	// 类名带进程号：同一台机器上开两个实例时，同名窗口类会让第二个实例注册失败。
	a.className = fmt.Sprintf("KeyboardStatisticsWnd_%d", os.Getpid())

	inst, err := registerClass(a.className, syscall.NewCallback(a.wndProc))
	if err != nil {
		return err
	}

	h, err := createWindow(a.className, windowTitle, clientWidth, clientHeight, inst)
	if err != nil {
		return err
	}
	a.hwnd = h

	// 边框厚度 = 窗口矩形 - 客户区矩形：窗口居中与最小尺寸都要靠它换算。
	wr, cr := windowRect(h), clientRect(h)
	a.frameW = wr.Width() - cr.Width()
	a.frameH = wr.Height() - cr.Height()

	dpi := dpiForWindow(h)
	if dpi <= 0 {
		dpi = 96
	}
	a.scale = float64(dpi) / 96.0

	// 换上自己的图标：不换的话标题栏和 Alt+Tab 里是系统默认的白板图标。
	// 尺寸跟着 DPI 走，高分屏下才不会被系统再缩一次（那就是糊的原因）。
	setWindowIcon(h, loadWindowIcon(px(16, a.scale)), loadWindowIcon(px(32, a.scale)))

	a.createFonts()

	sw, sh := screenSize()
	w := px(clientWidth, a.scale) + a.frameW
	hgt := px(clientHeight, a.scale) + a.frameH
	setWindowPos(h, (sw-w)/2, (sh-hgt)/2, w, hgt, swpNoZOrder)

	setTimer(h, refreshTimerID, uint32(refreshInterval/time.Millisecond))
	return nil
}

func (a *App) cleanup() {
	// 先摘托盘图标：进程结束后系统不会替我们清理，图标会一直残留到鼠标划过。
	a.trayRemove()
	killTimer(a.hwnd, refreshTimerID)
	close(a.stopped)
	for _, f := range a.fonts {
		if f != 0 {
			deleteObject(f)
		}
	}
}

func (a *App) show() {
	// 自启拉起时用 SW_SHOWMINNOACTIVE：最小化、不抢焦点，但任务栏里可见。
	cmd := int32(swShowNormal)
	if a.opt.StartMinimized {
		cmd = swShowMinNoActivate
	}
	showWindow(a.hwnd, cmd)
	updateWindow(a.hwnd)
}

// wndProc 是窗口过程。
//
// lParam 声明成 unsafe.Pointer 而不是 uintptr：WM_GETMINMAXINFO 之类的消息里
// 它本身就是指针，直接用 unsafe.Pointer 接住再转成结构体指针，既不违反 unsafe 的规则
// 也不会触发 go vet 的「uintptr 转指针」告警。
func (a *App) wndProc(h syscall.Handle, message uint32, wParam uintptr, lParam unsafe.Pointer) uintptr {
	// 安装器升级前发来的握手消息：让程序自己体面退出（会把缓存刷进库），
	// 而不是等 Restart Manager 等不下去再强杀。
	if a.wmQuitRequest != 0 && message == a.wmQuitRequest {
		a.quit()
		return 0
	}
	switch message {
	case wmPaint:
		a.paintWindow(h)
		return 0
	case wmEraseBkgnd:
		// 自己整块重绘（双缓冲），别让系统再刷一遍底色，否则会闪。
		return 1
	case wmTimer:
		if wParam == refreshTimerID {
			a.requestRefresh()
		}
		return 0
	case msgSnapshotReady:
		a.applyPendingSnapshot()
		return 0
	case wmSize:
		invalidate(h)
		return 0
	case wmGetMinMaxInfo:
		scale := a.scale
		if scale <= 0 {
			scale = 1
		}
		mmi := (*minMaxInfo)(lParam)
		mmi.MinTrackSize = point{
			px(minClientWidth, scale) + a.frameW,
			px(minClientHeight, scale) + a.frameH,
		}
		return 0
	case wmMouseMove:
		a.onMouseMove(uintptr(lParam))
		return 0
	case wmMouseLeave:
		a.setHover(hoverTarget{})
		return 0
	case wmLButtonDown:
		a.onClick(uintptr(lParam))
		return 0
	case msgTray:
		a.onTrayMessage(uintptr(lParam))
		return 0
	case wmClose:
		// 托盘没放上就只能真退出：否则既没窗口也没图标，用户只能去任务管理器。
		if !a.trayAdded {
			destroyWindow(h)
			return 0
		}
		switch a.closeAction {
		case config.CloseExit:
			a.quit()
		case config.CloseToTray:
			a.hideToTray()
		default:
			// 还没选过：问一次，记住答案，下次不再打扰。
			a.askCloseAction()
		}
		return 0
	case wmDestroy:
		postQuitMessage(0)
		return 0
	}
	return defWindowProc(h, message, wParam, uintptr(lParam))
}

func lParamX(lParam uintptr) int32 { return int32(int16(lParam & 0xFFFF)) }
func lParamY(lParam uintptr) int32 { return int32(int16((lParam >> 16) & 0xFFFF)) }

// layout 按当前客户区与 DPI 算一次布局。绘制与命中测试都用它，保证两边永远一致。
func (a *App) layout() Layout {
	return ComputeLayout(clientRect(a.hwnd), a.scale, len(stats.AllScopes()), len(tabTitles))
}

func (a *App) onMouseMove(lParam uintptr) {
	x, y := lParamX(lParam), lParamY(lParam)
	target := hoverTarget{}

	l := a.layout()
	if i, ok := l.HitScope(x, y); ok {
		target = hoverTarget{kind: hoverScope, index: i}
	} else if i, ok := l.HitTab(x, y); ok {
		target = hoverTarget{kind: hoverTab, index: i}
	} else if a.page == pageHeatmap {
		if i, ok := ComputeHeatmapLayout(l.Content, a.scale).HitKey(x, y); ok {
			target = hoverTarget{kind: hoverHeatmap, index: i}
		}
	} else if a.page == pageSettings {
		if hit := ComputeSettingsLayout(l.Content, a.scale).Hit(x, y); hit != SettingsHitNone {
			target = hoverTarget{kind: hoverSettings, index: int(hit)}
		}
	}

	// 第一次移动后注册离开通知，否则鼠标移出窗口后高亮会一直残留。
	trackMouseLeave(a.hwnd)
	a.setHover(target)
}

func (a *App) setHover(t hoverTarget) {
	if a.hover == t {
		return
	}
	a.hover = t
	invalidate(a.hwnd)
}

func (a *App) onClick(lParam uintptr) {
	x, y := lParamX(lParam), lParamY(lParam)
	l := a.layout()

	if i, ok := l.HitScope(x, y); ok {
		if scopes := stats.AllScopes(); i < len(scopes) {
			a.setScope(scopes[i])
		}
		return
	}
	if i, ok := l.HitTab(x, y); ok {
		a.setPage(pageID(i))
		return
	}
	if a.page != pageSettings {
		return
	}

	switch ComputeSettingsLayout(l.Content, a.scale).Hit(x, y) {
	case SettingsHitCollect:
		a.toggleCollecting()
	case SettingsHitAutostart:
		a.toggleAutostart()
	case SettingsHitExport:
		a.exportCSV()
	case SettingsHitClear:
		a.clearData()
	case SettingsHitOpenDataDir:
		a.openDataDir()
	case SettingsHitChangeDataDir:
		a.changeDataDir()
	}
}

func (a *App) setScope(s stats.Scope) {
	if a.scope == s {
		return
	}
	a.scope = s
	a.requestRefresh()
	invalidate(a.hwnd)
}

func (a *App) setPage(p pageID) {
	if a.page == p || p >= pageID(len(tabTitles)) {
		return
	}
	a.page = p
	a.hover = hoverTarget{}
	invalidate(a.hwnd)
}

func (a *App) toggleCollecting() {
	a.svc.SetCollecting(!a.svc.Collecting())
	a.collecting = a.svc.Collecting()
	// 托盘提示是窗口隐藏后唯一的状态出口，切完必须同步。
	a.traySetTip(a.collecting)
	a.requestRefresh()
	invalidate(a.hwnd)
}

// requestRefresh 把一次取数丢给后台协程；上一次还没回来就直接跳过本次 tick，
// 避免大范围聚合慢的时候请求堆积。
func (a *App) requestRefresh() {
	a.mu.Lock()
	if a.fetching || a.hwnd == 0 {
		a.mu.Unlock()
		return
	}
	a.fetching = true
	scope := a.scope
	hwnd := a.hwnd
	a.mu.Unlock()

	go func() {
		res := &snapshotResult{scope: scope, at: time.Now(), collecting: a.svc.Collecting()}
		// 这里只查库、不碰窗口：查询再慢也只慢在这个协程里，窗口线程不受影响。
		res.snap, res.err = a.svc.Snapshot(scope)

		a.mu.Lock()
		a.pending = res
		a.mu.Unlock()

		postMessage(hwnd, msgSnapshotReady, 0, 0)
	}()
}

func (a *App) applyPendingSnapshot() {
	a.mu.Lock()
	res := a.pending
	a.pending = nil
	a.fetching = false
	a.mu.Unlock()

	if res == nil {
		return
	}
	if res.scope != a.scope {
		// 取数期间用户切了时间范围，这份数据已经过时，重取而不是画错数据。
		a.requestRefresh()
		return
	}

	if res.err != nil {
		a.snapErr = res.err.Error()
	} else {
		a.snap = res.snap
		a.snapErr = ""
		a.lastRefresh = res.at
	}
	a.collecting = res.collecting
	// 这里特地去掉了「每次刷新都从注册表重读自启状态」：那是每 2 秒 3 次注册表读，
	// 而注册表只会被本进程改动（启动时同步一次、用户点复选框时同步一次就够了）。
	// 界面线程上任何一次多余的系统调用，都是「未响应」的潜在来源。
	invalidate(a.hwnd)
}

func (a *App) autostart() *config.Autostart {
	if a.opt.Autostart == nil || a.opt.ExePath == "" {
		return nil
	}
	return config.NewAutostart(a.opt.Autostart, a.opt.ExePath)
}

// syncSettingsState 把注册表里的自启状态同步到界面。
//
// 期间 updating 为真：自绘复选框没有「程序化勾选」这种事件，这个标志是防御性保险，
// 将来若换成真正的 Win32 控件，也不会因为 SetCheck 反过来写一遍注册表。
func (a *App) syncSettingsState() {
	s := &a.settings
	s.updating = true
	defer func() { s.updating = false }()

	mgr := a.autostart()
	if mgr == nil {
		s.autostartAvailable = false
		s.autostartChecked = false
		s.autostartStale = false
		return
	}
	s.autostartAvailable = true

	enabled, err := mgr.Enabled()
	s.autostartChecked = err == nil && enabled

	stale, err := mgr.Stale()
	s.autostartStale = err == nil && stale
}

func (a *App) toggleAutostart() {
	mgr := a.autostart()
	if mgr == nil {
		// 注册表不可用（或拿不到自身路径）时复选框是禁用态，这里只做兜底。
		return
	}

	switch autostartAction(a.settings.updating, !a.settings.autostartChecked) {
	case autostartEnable:
		if err := mgr.Enable(); err != nil {
			a.showError(err)
			return
		}
	case autostartDisable:
		if err := mgr.Disable(); err != nil {
			a.showError(err)
			return
		}
	}

	a.syncSettingsState()
	invalidate(a.hwnd)
}

func (a *App) exportCSV() {
	dir, ok := browseForFolder(a.hwnd, "选择导出 CSV 的文件夹")
	if !ok {
		return
	}

	files, err := a.svc.ExportCSV(dir)
	if err != nil {
		a.showError(fmt.Errorf("导出 CSV: %w", err))
		return
	}

	messageBox(a.hwnd,
		"已生成 "+strconv.Itoa(len(files))+" 个文件：\n\n"+strings.Join(files, "\n"),
		"导出完成", mbOK|mbIconInfo)
}

func (a *App) clearData() {
	// 默认按钮放在「否」，避免手快按回车就把数据清了。
	answer := messageBox(a.hwnd,
		"这会删除全部统计数据，且无法恢复。\n\n确定继续吗？",
		"清空全部数据", mbYesNo|mbIconWarning|mbDefButton2)
	if answer != idYes {
		return
	}

	if err := a.svc.ClearData(); err != nil {
		a.showError(fmt.Errorf("清空统计数据: %w", err))
		return
	}

	messageBox(a.hwnd, "统计数据已全部删除。", "已清空", mbOK|mbIconInfo)
	a.requestRefresh()
	invalidate(a.hwnd)
}

func (a *App) openDataDir() {
	dir := a.opt.DataDir
	if dir == "" {
		a.showError(fmt.Errorf("无法定位数据目录"))
		return
	}
	// 目录可能还不存在（用户从没启动过采集），先建出来再打开。
	if err := os.MkdirAll(dir, 0o755); err != nil {
		a.showError(fmt.Errorf("创建数据目录: %w", err))
		return
	}
	if err := shellOpen(dir); err != nil {
		a.showError(err)
	}
}

// changeDataDir 让用户挑一个新目录放统计数据。
//
// 这里只写设置、不搬文件：此刻数据库正开着，Windows 不允许搬走被占用的文件，
// 强行先关库再搬会让界面停在「服务已关闭」的半死状态。
// 真正的迁移放到下次启动、数据库尚未打开时做（见 cmd/app/main.go）。
func (a *App) changeDataDir() {
	if a.opt.SettingsStore == nil {
		messageBox(a.hwnd, "当前环境无法保存设置，暂时不能更改数据目录。",
			"更改数据目录", mbOK|mbIconWarning)
		return
	}

	dir, ok := browseForFolder(a.hwnd, "选择新的数据目录")
	if !ok {
		return
	}
	dir = strings.TrimSpace(dir)
	if dir == "" || strings.EqualFold(filepath.Clean(dir), filepath.Clean(a.opt.DataDir)) {
		return
	}

	// 先验证再询问：让用户在「确定」之前就知道这个目录能不能用。
	if err := config.EnsureWritable(dir); err != nil {
		a.showError(fmt.Errorf("这个目录不能写：\n%v", err))
		return
	}
	dbPath := filepath.Join(dir, "stats.db")
	if _, err := os.Stat(dbPath); err == nil {
		a.showError(fmt.Errorf("这个目录里已经有统计数据了：\n%s\n\n请换一个空目录。", dbPath))
		return
	}

	answer := messageBox(a.hwnd, fmt.Sprintf(
		"数据目录将改为：\n\n%s\n\n"+
			"现有统计数据会在下次启动时自动搬过去。\n"+
			"设置完成后程序会退出，请重新打开。\n\n确定要更改吗？", dir),
		"更改数据目录", mbYesNo|mbIconInfo|mbDefButton1)
	if answer != idYes {
		return
	}

	if err := config.SetDataDir(a.opt.SettingsStore, dir); err != nil {
		a.showError(fmt.Errorf("保存数据目录设置: %w", err))
		return
	}

	messageBox(a.hwnd, "数据目录已设为：\n\n"+dir+"\n\n程序即将退出，请重新打开以完成迁移。",
		"更改数据目录", mbOK|mbIconInfo)
	// 这里必须真退出：迁移只在下次启动、数据库尚未打开时进行。
	// 用 WM_CLOSE 会被托盘常驻解释成「收进托盘」，迁移就永远等不到。
	a.quit()
}

func (a *App) showError(err error) {
	messageBox(a.hwnd, err.Error(), windowTitle, mbOK|mbIconError)
}
