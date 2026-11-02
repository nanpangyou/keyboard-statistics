package stats

import "fmt"

// KeyName 返回键码的显示名。口径见 docs/metrics.md 第 2 节。
//
// 同一个 vk 在任何界面、导出、组合键名称里都必须得到同一个字符串。
func KeyName(vk uint16) string {
	// 字母 A-Z
	if vk >= 0x41 && vk <= 0x5A {
		return string(rune('A' + vk - 0x41))
	}
	// 主键盘数字 0-9
	if vk >= 0x30 && vk <= 0x39 {
		return string(rune(vk))
	}
	// 小键盘数字 0-9
	if vk >= 0x60 && vk <= 0x69 {
		return "小键盘" + string(rune('0'+vk-0x60))
	}
	// 功能键 F1-F24
	if vk >= 0x70 && vk <= 0x87 {
		return fmt.Sprintf("F%d", vk-0x70+1)
	}
	if name, ok := namedKeys[vk]; ok {
		return name
	}
	if name, ok := oemKeys[vk]; ok {
		return name
	}
	return fmt.Sprintf("VK_%d", vk)
}

// namedKeys 是非字符类的固定名字。左右修饰键映射到同一个显示名（口径要求合并）。
var namedKeys = map[uint16]string{
	VKBack:     "退格",
	VKTab:      "Tab",
	VKEnter:    "回车",
	VKShift:    "Shift",
	VKLShift:   "Shift",
	VKRShift:   "Shift",
	VKControl:  "Ctrl",
	VKLControl: "Ctrl",
	VKRControl: "Ctrl",
	VKMenu:     "Alt",
	VKLMenu:    "Alt",
	VKRMenu:    "Alt",
	VKLWin:     "Win",
	VKRWin:     "Win",
	VKApps:     "菜单键",
	VKCaps:     "CapsLock",
	VKEscape:   "Esc",
	VKSpace:    "空格",
	VKPageUp:   "PageUp",
	VKPageDn:   "PageDown",
	VKEnd:      "End",
	VKHome:     "Home",
	VKLeft:     "←",
	VKUp:       "↑",
	VKRight:    "→",
	VKDown:     "↓",
	VKPrint:    "PrintScreen",
	VKInsert:   "Insert",
	VKDelete:   "Delete",
	VKPause:    "Pause",
	VKNumLock:  "NumLock",
}

// oemKeys 是主键盘标点，取 US 布局下的未加 Shift 字符。
var oemKeys = map[uint16]string{
	VKOEM1:     ";",
	VKOEMPlus:  "=",
	VKOEMComma: ",",
	VKOEMMinus: "-",
	VKOEMDot:   ".",
	VKOEM2:     "/",
	VKOEM3:     "`",
	VKOEM4:     "[",
	VKOEM5:     "\\",
	VKOEM6:     "]",
	VKOEM7:     "'",
	VKOEM102:   "\\",
}
