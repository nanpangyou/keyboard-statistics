//go:build !windows

package ui

import (
	"keyboard-statistics/internal/config"
	"keyboard-statistics/internal/service"
)

// 本文件是界面层在非 Windows 平台的占位实现。
//
// 界面本身只跑在 Windows 上，但 AGENTS.md 要求 `go test ./...` 在任何平台都能编译通过，
// 所以这里保留与 Windows 版一致的对外 API，Run 直接返回。

// Options 是界面初始化参数，字段与 Windows 版保持一致。
type Options struct {
	Version        string
	ExePath        string
	Autostart      config.Registry
	StartMinimized bool
}

// App 在非 Windows 平台没有窗口，只用来让调用方代码保持可编译。
type App struct{}

// New 返回一个空壳 App。
func New(opts Options, svc *service.Service) *App { return &App{} }

// Run 立即返回：没有窗口可跑。
func (a *App) Run() {}
