// 键盘使用统计：只在本机记录按键频次与时间的 Windows 小工具。
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"keyboard-statistics/internal/config"
	"keyboard-statistics/internal/service"
	"keyboard-statistics/internal/ui"
)

// version 由构建脚本通过 -ldflags 注入。
var version = "0.1.0-dev"

// maxLogSize 超过就轮转，避免日志无限长。
const maxLogSize = 5 << 20

func main() {
	showVersion := flag.Bool("version", false, "打印版本号后退出")
	autostart := flag.Bool("autostart", false, "由开机自启拉起：启动后最小化窗口")
	flag.Parse()

	if *showVersion {
		fmt.Println("keyboard-statistics", version)
		return
	}

	dir, err := config.EnsureDir()
	if err != nil {
		fatal(nil, err)
	}

	logger, closeLog := newLogger(dir)
	defer closeLog()

	svc, err := service.New(service.Options{
		DBPath:    filepath.Join(dir, "stats.db"),
		StartHook: true,
	})
	if err != nil {
		fatal(logger, err)
	}

	if err := svc.Start(); err != nil {
		fatal(logger, err)
	}

	exePath, err := os.Executable()
	if err != nil {
		logger.Warn("获取自身路径失败，开机自启可能不可用", "error", err)
	}

	reg, err := config.OpenRunRegistry()
	if err != nil {
		logger.Warn("打开注册表失败，开机自启不可用", "error", err)
		reg = nil
	}

	application := ui.New(ui.Options{
		Version:        version,
		ExePath:        exePath,
		Autostart:      reg,
		StartMinimized: *autostart,
	}, svc)

	application.Run()

	// 窗口关闭即退出：v0.1 没有托盘常驻，关窗就该停止采集
	if err := svc.Close(); err != nil {
		logger.Error("退出时关闭服务失败", "error", err)
	}
}

func fatal(logger *slog.Logger, err error) {
	if logger != nil {
		logger.Error("启动失败", "error", err)
	}
	os.Exit(1)
}

// newLogger 建一个写文件的 slog，按大小做一次简单轮转。
// 用 -H windowsgui 编译后没有控制台，日志是唯一的排障手段。
// 返回的第二个值负责关闭文件。
func newLogger(dir string) (*slog.Logger, func()) {
	path := filepath.Join(dir, "app.log")

	if fi, err := os.Stat(path); err == nil && fi.Size() > maxLogSize {
		_ = os.Remove(path + ".1")
		_ = os.Rename(path, path+".1")
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		// 日志都写不了也不要拖着程序不起，退化到丢弃日志
		return slog.New(slog.NewTextHandler(discard{}, nil)), func() {}
	}

	handler := slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(handler), func() { _ = f.Close() }
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
