package config

import (
	"errors"
	"strings"
)

// closeActionSetting 是注册表里保存「点关闭按钮时怎么办」的值名。
//
// 放在设置存储（HKCU\Software\keyboard-statistics）里，而不是数据库 meta：
// 它是界面偏好，跟着用户账号走，和统计数据无关；换数据目录也不该丢。
const closeActionSetting = "CloseAction"

// CloseAction 是用户选择的关窗行为。
type CloseAction int

const (
	// CloseAsk 表示还没选过：关窗时弹一次选项框，然后把结果记下来。
	CloseAsk CloseAction = iota
	// CloseToTray 表示收进托盘，继续统计。
	CloseToTray
	// CloseExit 表示直接退出程序。
	CloseExit
)

// String 返回存进设置里的值。
func (a CloseAction) String() string {
	switch a {
	case CloseToTray:
		return "tray"
	case CloseExit:
		return "exit"
	}
	return ""
}

// ToTray 表示关窗时是否收进托盘。
//
// CloseAsk 也返回 true：真正问用户是在关窗那一刻，而在此之前
// 托盘菜单上的勾选状态得有个合理默认——收进托盘是托盘常驻的默认语义。
func (a CloseAction) ToTray() bool {
	return a != CloseExit
}

// ParseCloseAction 把存下来的字符串解析成 CloseAction。
// 认不出来的值一律当成 CloseAsk：宁可多问一次，也不要猜错用户的意思。
func ParseCloseAction(s string) CloseAction {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "tray":
		return CloseToTray
	case "exit":
		return CloseExit
	}
	return CloseAsk
}

// GetCloseAction 读用户选择。读失败或存储不可用时返回 CloseAsk，不阻断关窗。
func GetCloseAction(store SettingStore) CloseAction {
	if store == nil {
		return CloseAsk
	}
	v, ok, err := store.Get(closeActionSetting)
	if err != nil || !ok {
		return CloseAsk
	}
	return ParseCloseAction(v)
}

// SetCloseAction 记住用户的选择。
func SetCloseAction(store SettingStore, action CloseAction) error {
	if store == nil {
		return errors.New("没有可用的设置存储")
	}
	if err := store.Set(closeActionSetting, action.String()); err != nil {
		return err
	}
	return nil
}
