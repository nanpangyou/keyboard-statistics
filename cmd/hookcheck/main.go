// 手工/自动验证程序：安装真实键盘钩子，注入按键，验证「钩子 → 采集 → 聚合」整条链路。
// 用法：go run ./cmd/hookcheck
package main

import (
	"fmt"
	"os"
	"sort"
	"syscall"
	"time"

	"keyboard-statistics/internal/collect"
	"keyboard-statistics/internal/hook"
	"keyboard-statistics/internal/stats"
)

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	procKeybdEvent  = user32.NewProc("keybd_event")
	procMessageBoxW = user32.NewProc("MessageBoxW")
)

const (
	keyeventfKeyUp = 0x0002

	vkControl = 0x11
	vkA       = 0x41
	vkC       = 0x43
	vkShift   = 0x10
	vkLeft    = 0x25 // Shift+← 应该算组合
)

func keyDown(vk uint16) { procKeybdEvent.Call(uintptr(vk), 0, 0, 0) }
func keyUp(vk uint16)   { procKeybdEvent.Call(uintptr(vk), 0, keyeventfKeyUp, 0) }

func tap(vk uint16) {
	keyDown(vk)
	time.Sleep(15 * time.Millisecond)
	keyUp(vk)
	time.Sleep(15 * time.Millisecond)
}

func main() {
	c := collect.New(nil)

	h, err := hook.Start(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "启动钩子失败:", err)
		os.Exit(1)
	}

	// 注入四组输入，期望：A×1、C×1、Ctrl×1、Shift×1、←×1，
	// 其中 Ctrl+C 与 Shift+← 应各产生 1 条组合。
	tap(vkA)

	keyDown(vkControl)
	tap(vkC)
	keyUp(vkControl)
	time.Sleep(15 * time.Millisecond)

	keyDown(vkShift)
	tap(vkLeft)
	keyUp(vkShift)
	time.Sleep(15 * time.Millisecond)

	// 长按：连发 5 次 KEYDOWN 不松开，只该计 1 次
	keyDown(vkA)
	for i := 0; i < 4; i++ {
		keyDown(vkA)
		time.Sleep(10 * time.Millisecond)
	}
	keyUp(vkA)

	time.Sleep(200 * time.Millisecond) // 等钩子线程处理完

	if err := h.Stop(); err != nil {
		fmt.Fprintln(os.Stderr, "停止钩子失败:", err)
	}

	b := c.Flush()

	fmt.Println("=== 单键 ===")
	keys := make([]string, 0, len(b.Keys))
	total := int64(0)
	for _, d := range b.Keys {
		keys = append(keys, fmt.Sprintf("  %-8s %d", stats.KeyName(d.VK), d.Count))
		total += d.Count
	}
	sort.Strings(keys)
	for _, s := range keys {
		fmt.Println(s)
	}

	fmt.Println("=== 组合键 ===")
	combos := make([]string, 0, len(b.Combos))
	for _, d := range b.Combos {
		combos = append(combos, fmt.Sprintf("  %-16s %d", d.Combo, d.Count))
	}
	sort.Strings(combos)
	for _, s := range combos {
		fmt.Println(s)
	}

	fmt.Printf("合计 %d 次按键、%d 种组合\n", total, len(b.Combos))

	if len(b.Combos) != 2 {
		fmt.Fprintln(os.Stderr, "❌ 期望 2 种组合（Ctrl+C、Shift+←）")
		os.Exit(1)
	}
	fmt.Println("✅ 钩子链路正常")
}
