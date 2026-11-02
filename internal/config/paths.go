// Package config 负责本地路径解析与开机自启管理。
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const appDirName = "keyboard-statistics"

// Dir 返回数据目录：%LOCALAPPDATA%\keyboard-statistics。
//
// 刻意不写程序所在目录：exe 可能被放在 Program Files 或只读位置，
// 而且用户重装/移动 exe 时数据不该跟着丢。
func Dir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("定位用户数据目录: %w", err)
		}
	}
	return filepath.Join(base, appDirName), nil
}

// EnsureDir 确保数据目录存在并返回它。
func EnsureDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建数据目录 %s: %w", dir, err)
	}
	return dir, nil
}

// DBPath 返回统计库路径。
func DBPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "stats.db"), nil
}

// LogPath 返回日志文件路径。
func LogPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "app.log"), nil
}
