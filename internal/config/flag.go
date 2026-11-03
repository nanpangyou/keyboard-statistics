package config

import (
	"errors"
	"strings"
)

// FlagTrayHintShown 记录「首次收进托盘的那条气泡提示已经弹过了」。
//
// 存下来而不是只记在内存里：用户要的是「只提示第一次」，每次启动都弹一遍
// 就变成骚扰了。
const FlagTrayHintShown = "TrayHintShown"

// GetFlag 读一个只关心「有没有设过」的开关。
//
// 读失败返回 false：这类标记只影响要不要多说一句话，不值得为它打断启动。
func GetFlag(store SettingStore, name string) bool {
	if store == nil {
		return false
	}
	v, ok, err := store.Get(name)
	if err != nil || !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "0", "false", "no":
		return false
	}
	return true
}

// SetFlag 记下一个开关。
func SetFlag(store SettingStore, name string, value bool) error {
	if store == nil {
		return errors.New("没有可用的设置存储")
	}
	v := "0"
	if value {
		v = "1"
	}
	if err := store.Set(name, v); err != nil {
		return err
	}
	return nil
}
