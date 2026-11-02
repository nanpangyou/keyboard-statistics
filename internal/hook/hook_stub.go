//go:build !windows

// Package hook 提供非 Windows 平台的空实现，只为让 go test ./... 能跑通。
package hook

import "errors"

// Handler 接收按键事件。
type Handler interface {
	KeyDown(vk uint16)
	KeyUp(vk uint16)
}

// Hook 在非 Windows 平台上永远是 nil 实例。
type Hook struct{}

// Start 在非 Windows 平台上直接报错：本项目只支持 Windows。
func Start(h Handler) (*Hook, error) {
	return nil, errors.New("hook: 键盘钩子仅支持 Windows")
}

// Stop 在非 Windows 平台上什么也不做。
func (h *Hook) Stop() error { return nil }
