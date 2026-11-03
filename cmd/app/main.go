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
	"keyboard-statistics/internal/single"
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

	// exe 路径先拿到：数据目录解析要靠它判断是否处于便携模式。
	exePath, exeErr := os.Executable()

	// 设置存储打开失败不阻断启动：数据目录退回默认位置，程序照样能用。
	store, storeErr := config.OpenSettingsStore()
	if storeErr != nil {
		store = nil
	}

	dir, source, err := config.ResolveDataDir(exePath, store)
	if err != nil {
		ui.ShowFatal("无法确定数据目录：\n\n" + err.Error())
		os.Exit(1)
	}

	logger, closeLog := newLogger(dir)
	defer closeLog()

	if exeErr != nil {
		logger.Warn("获取自身路径失败，便携模式与开机自启可能不可用", "error", exeErr)
	}
	if storeErr != nil {
		logger.Warn("打开设置存储失败，自定义数据目录不可用", "error", storeErr)
	}

	// 单实例：两个进程同时写同一个 SQLite 库会互相打架，升级时也说不清该关哪个。
	guard, alreadyRunning, lockErr := single.Acquire(single.DefaultName)
	switch {
	case lockErr != nil:
		// 拿不到锁不把程序打死：正常机器上几乎不会发生，
		// 真发生了也宁可让用户能用，只留一条日志。
		logger.Warn("单实例检查失败，继续启动", "error", lockErr)
	case alreadyRunning:
		logger.Info("已有实例在运行，本次启动直接退出", "autostart", *autostart)
		// 自启拉起时既不抢焦点也不提示：开机弹框会打断用户，而且多半没人看着。
		// 手动双击时一定要有反馈，否则「点了图标却没反应」会让人以为程序坏了。
		if !*autostart {
			if ui.ActivateExisting() {
				ui.ShowInfo("键盘使用统计已经在运行了。\n\n已经把它的窗口切到前面来了。")
			} else {
				ui.ShowInfo("键盘使用统计已经在运行了。\n\n它可能被收进了通知区域（托盘），" +
					"在右下角找到图标点一下就能看到窗口。")
			}
		}
		return
	default:
		defer guard.Release()
	}

	if err := config.EnsureWritable(dir); err != nil {
		ui.ShowFatal(fmt.Sprintf(
			"数据目录无法写入：\n\n%s\n\n%s\n\n"+
				"请在设置里换一个你有写权限的目录，或把程序装到非 Program Files 的位置。",
			dir, err.Error()))
		os.Exit(1)
	}
	logger.Info("数据目录", "path", dir, "source", source.String())

	// 用户改过数据目录时，趁数据库还没打开把旧数据搬过来。
	// 运行中迁移要先把库关掉再动文件，容易留下半死状态，启动时做最安全。
	if source == config.SourceSetting {
		if fallback, _, ferr := config.FallbackDir(exePath); ferr == nil {
			if moved, merr := config.MigrateIfNeeded(fallback, dir); merr != nil {
				logger.Warn("迁移旧数据失败，继续使用新目录", "from", fallback, "error", merr)
			} else if moved {
				logger.Info("已把旧数据迁移到新目录", "from", fallback, "to", dir)
			}
		}
	}

	svc, err := service.New(service.Options{
		DBPath:    config.DBPathIn(dir),
		StartHook: true,
	})
	if err != nil {
		logger.Error("打开统计库失败", "error", err)
		ui.ShowFatal("统计数据无法打开：\n\n" + err.Error())
		os.Exit(1)
	}

	if err := svc.Start(); err != nil {
		logger.Error("启动采集失败", "error", err)
		ui.ShowFatal("键盘采集启动失败：\n\n" + err.Error())
		os.Exit(1)
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
		DataDir:        dir,
		DataDirSource:  source.String(),
		SettingsStore:  store,
		// 界面卡死时把现场写进日志：进程从外面看只是「在等」，只有栈能说明堵在哪。
		OnUIStall: func(dump string) { logger.Error("界面无响应", "stack", dump) },
	}, svc)

	application.Run()

	// 窗口关闭即退出：v0.1 没有托盘常驻，关窗就该停止采集
	if err := svc.Close(); err != nil {
		logger.Error("退出时关闭服务失败", "error", err)
	}
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
