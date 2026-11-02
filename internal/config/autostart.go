package config

import (
	"fmt"
	"strings"
)

// runValueName 是注册表里自启项的名字，固定不变，便于用户卸载时手动清理。
const runValueName = "KeyboardStatistics"

// Registry 抽象「读写单个字符串值」，真实实现是 HKCU\...\Run，测试用内存实现。
type Registry interface {
	Get(name string) (value string, ok bool, err error)
	Set(name, value string) error
	Delete(name string) error
}

// Autostart 管理开机自启。
//
// 按红线要求：自启只能由用户主动勾选触发，程序绝不自行写入；
// 取消勾选必须真的删掉注册表值，不留残渣。
type Autostart struct {
	reg Registry
	exe string
}

// NewAutostart 绑定一个注册表实现与当前 exe 全路径。
func NewAutostart(reg Registry, exe string) *Autostart {
	return &Autostart{reg: reg, exe: exe}
}

// CommandLine 生成注册表里保存的命令行。
// 必须带引号，否则路径里有空格时 Windows 会按空格截断。
func CommandLine(exe string) string {
	return `"` + exe + `" --autostart`
}

// ParseCommandExe 从注册表值里取出 exe 路径，兼容历史遗留的无引号写法。
func ParseCommandExe(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if value[0] == '"' {
		if i := strings.IndexByte(value[1:], '"'); i >= 0 {
			return value[1 : 1+i]
		}
		return strings.Trim(value, `"`)
	}
	if i := strings.IndexByte(value, ' '); i > 0 {
		return value[:i]
	}
	return value
}

// Enabled 报告当前是否已启用自启。
func (a *Autostart) Enabled() (bool, error) {
	_, ok, err := a.reg.Get(runValueName)
	if err != nil {
		return false, fmt.Errorf("读取自启注册表项: %w", err)
	}
	return ok, nil
}

// Enable 写入自启项。
func (a *Autostart) Enable() error {
	if err := a.reg.Set(runValueName, CommandLine(a.exe)); err != nil {
		return fmt.Errorf("写入自启注册表项: %w", err)
	}
	return nil
}

// Disable 删除自启项；本来就没有也算成功。
func (a *Autostart) Disable() error {
	if err := a.reg.Delete(runValueName); err != nil {
		return fmt.Errorf("删除自启注册表项: %w", err)
	}
	return nil
}

// Stale 报告注册表里的路径是否已经指向别处（用户移动/重命名了 exe）。
// 未启用自启时永远返回 false——那不是「失效」，是「没开」。
func (a *Autostart) Stale() (bool, error) {
	value, ok, err := a.reg.Get(runValueName)
	if err != nil {
		return false, fmt.Errorf("读取自启注册表项: %w", err)
	}
	if !ok {
		return false, nil
	}
	return !strings.EqualFold(ParseCommandExe(value), a.exe), nil
}
