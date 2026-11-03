//go:build windows

package config

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// runKeyPath 固定在 HKCU：写 HKLM 需要管理员权限，且会影响本机所有用户。
const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// settingsKeyPath 是程序自己的设置键，目前存「自定义数据目录」。
// 同样放 HKCU：数据目录是每个用户自己的选择，不该影响本机其他用户。
const settingsKeyPath = `Software\keyboard-statistics`

// OpenRunRegistry 打开当前用户的 Run 键。
func OpenRunRegistry() (Registry, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath,
		registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return nil, fmt.Errorf("打开自启注册表键: %w", err)
	}
	return &runKeyRegistry{key: key}, nil
}

// OpenSettingsStore 打开当前用户的程序设置键。
// 键不存在时创建——首次运行就要能写数据目录，不该因为键不存在而失败。
func OpenSettingsStore() (SettingStore, error) {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, settingsKeyPath,
		registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return nil, fmt.Errorf("打开设置注册表键: %w", err)
	}
	return &runKeyRegistry{key: key}, nil
}

type runKeyRegistry struct {
	key registry.Key
}

func (r *runKeyRegistry) Get(name string) (string, bool, error) {
	value, _, err := r.key.GetStringValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("读取注册表值 %q: %w", name, err)
	}
	return value, true, nil
}

func (r *runKeyRegistry) Set(name, value string) error {
	if err := r.key.SetStringValue(name, value); err != nil {
		return fmt.Errorf("写入注册表值 %q: %w", name, err)
	}
	return nil
}

func (r *runKeyRegistry) Delete(name string) error {
	err := r.key.DeleteValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("删除注册表值 %q: %w", name, err)
	}
	return nil
}

// Close 释放注册表句柄。
func (r *runKeyRegistry) Close() error {
	if err := r.key.Close(); err != nil {
		return fmt.Errorf("关闭注册表键: %w", err)
	}
	return nil
}
