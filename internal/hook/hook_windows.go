//go:build windows

// Package hook 用 WH_KEYBOARD_LL 全局键盘钩子采集按键事件。
//
// 这里只做一件最轻的事：把 vkCode 转给 Handler。任何耗时的活（落库、UI）
// 都不许放进回调，否则用户能明显感觉到系统输入变慢。
package hook

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	whKeyboardLL = 13

	wmKeyDown    = 0x0100
	wmKeyUp      = 0x0101
	wmSysKeyDown = 0x0104
	wmSysKeyUp   = 0x0105
	wmQuit       = 0x0012

	stopTimeout = 3 * time.Second
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")

	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
)

// Handler 接收按键事件。实现必须立刻返回，不能阻塞。
type Handler interface {
	KeyDown(vk uint16)
	KeyUp(vk uint16)
}

// kbdllHookStruct 对应 KBDLLHOOKSTRUCT。
type kbdllHookStruct struct {
	vkCode      uint32
	scanCode    uint32
	flags       uint32
	time        uint32
	dwExtraInfo uintptr
}

// msgStruct 对应 Win32 的 MSG（x/sys/windows 没有导出它）。
// 字段顺序与对齐必须和系统结构体一致，否则 GetMessageW 会写坏内存。
type msgStruct struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

// Hook 是一个已安装的全局键盘钩子。
type Hook struct {
	handler  Handler
	handle   windows.Handle
	threadID uint32
	callback uintptr
	done     chan struct{}
	stopOnce sync.Once
	stopErr  error
}

// 回调是纯函数，只能通过包级变量拿到当前 Handler。
// 同一个进程装多个全局键盘钩子没有意义，所以这里只保留一个。
var (
	currentMu sync.RWMutex
	current   Handler
)

func setCurrent(h Handler) {
	currentMu.Lock()
	current = h
	currentMu.Unlock()
}

func getCurrent() Handler {
	currentMu.RLock()
	defer currentMu.RUnlock()
	return current
}

// Start 安装钩子并阻塞式地跑消息循环（在独立 goroutine 里）。
// 返回时钩子已确认安装成功；安装失败会直接返回错误，不会留下半死状态。
func Start(h Handler) (*Hook, error) {
	if h == nil {
		return nil, errors.New("hook: Handler 不能为 nil")
	}

	hk := &Hook{handler: h, done: make(chan struct{})}
	hk.callback = syscall.NewCallback(hk.proc)

	ready := make(chan error, 1)
	go hk.run(ready)

	if err := <-ready; err != nil {
		return nil, err
	}
	return hk, nil
}

func (h *Hook) run(ready chan<- error) {
	// 钩子必须装在「跑消息循环的那个线程」上，所以两者锁死在同一 OS 线程
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	h.threadID = windows.GetCurrentThreadId()
	setCurrent(h.handler)
	defer setCurrent(nil)

	handle, _, errno := procSetWindowsHookExW.Call(whKeyboardLL, h.callback, 0, 0)
	if handle == 0 {
		ready <- fmt.Errorf("安装键盘钩子失败（可能被安全软件拦截）: %w", errno)
		return
	}
	h.handle = windows.Handle(handle)
	defer procUnhookWindowsHookEx.Call(uintptr(h.handle))

	ready <- nil

	var msg msgStruct
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 { // 0 = WM_QUIT，-1 = 出错
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	close(h.done)
}

// proc 是钩子回调。这里只做最小的事：判定按下/松开并转发。
//
// lParam 声明成 unsafe.Pointer 而不是 uintptr：系统传进来的本来就是指针，
// 这样写既准确，也避免 uintptr→unsafe.Pointer 的转换被 go vet 判为可疑用法。
func (h *Hook) proc(nCode, wParam uintptr, lParam unsafe.Pointer) uintptr {
	if int32(nCode) >= 0 {
		if handler := getCurrent(); handler != nil {
			kb := (*kbdllHookStruct)(lParam)
			switch uint32(wParam) {
			case wmKeyDown, wmSysKeyDown:
				handler.KeyDown(uint16(kb.vkCode))
			case wmKeyUp, wmSysKeyUp:
				handler.KeyUp(uint16(kb.vkCode))
			}
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, nCode, wParam, uintptr(lParam))
	return ret
}

// Stop 卸载钩子并等待消息循环退出。重复调用安全。
func (h *Hook) Stop() error {
	h.stopOnce.Do(func() {
		procPostThreadMessageW.Call(uintptr(h.threadID), wmQuit, 0, 0)
		select {
		case <-h.done:
		case <-time.After(stopTimeout):
			h.stopErr = errors.New("hook: 停止超时，消息循环未退出")
		}
	})
	return h.stopErr
}
