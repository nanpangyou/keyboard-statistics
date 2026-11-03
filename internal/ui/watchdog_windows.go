//go:build windows

package ui

import (
	"fmt"
	"runtime"
	"time"
)

const (
	// stallProbeInterval 是心跳间隔：2 秒一次，卡顿基本当场就能被发现。
	stallProbeInterval = 2 * time.Second
	// stallProbeTimeoutMs 是单次探针等多久算没回应。
	// 比系统判定「未响应」的 5 秒短，所以日志会先于任务管理器出现记录。
	stallProbeTimeoutMs = 2000
	// stallDumpRepeat 是同一段卡顿里重复写栈的间隔。
	stallDumpRepeat = 30 * time.Second
)

// watchUIStall 盯着界面线程，卡住就把所有 goroutine 的栈写进日志。
//
// 为什么要专门做这件事：界面卡死时进程 CPU 是 0、线程状态都是 Wait，
// 和「正在安静地等消息」从外面看一模一样，事后无论怎么看进程列表都分辨不出来。
// goroutine 栈是唯一能指出「到底堵在哪一行」的证据，而它只有在程序还活着的时候才拿得到。
//
// 探针是 SendMessage 到窗口线程（消息号 WM_NULL，不改任何状态）：
// 它必须由窗口线程处理完才返回，所以「没在超时内返回」就等于界面线程没在转消息。
func (a *App) watchUIStall() {
	if a.opt.OnUIStall == nil {
		return
	}

	tr := newStallTracker(stallDumpRepeat)
	ticker := time.NewTicker(stallProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopped:
			return
		case now := <-ticker.C:
			// 窗口还没建好（或已经销毁）时 probe 必然失败，那不算卡顿。
			if a.hwnd == 0 {
				continue
			}
			responded := sendMessageTimeout(a.hwnd, wmNull, stallProbeTimeoutMs)
			if !tr.Observe(now, responded) {
				continue
			}

			// 先落库再写栈：界面卡死时用户多半会直接去任务管理器结束进程，
			// 那一刻内存里还没落库的计数就全没了。这台协程不卡，抢在强杀之前存下来。
			if err := a.svc.FlushNow(); err != nil {
				a.opt.OnUIStall(fmt.Sprintf("界面卡住期间落库失败：%v", err))
			}

			a.opt.OnUIStall(fmt.Sprintf(
				"界面线程卡住 %s（探针 %dms 无回应），goroutine 栈：\n%s",
				tr.StalledFor(now).Round(time.Millisecond), stallProbeTimeoutMs, allStacks()))
		}
	}
}

// allStacks 导出所有 goroutine 的调用栈，用于界面卡死后的现场取证。
func allStacks() string {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return string(buf[:n])
		}
		// 栈比缓冲区大时翻倍重来，宁可多占一点内存也不要截断掉关键那几帧。
		buf = make([]byte, len(buf)*2)
	}
}
