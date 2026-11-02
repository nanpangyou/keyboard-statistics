package stats

import "strings"

// ComboName 判断「在已按住 pressed 的情况下按下 key」是否构成组合键。
//
// 构成时返回规范化名称（修饰键固定 Ctrl → Shift → Alt → Win 顺序，主键在最后），
// 不构成时 ok 为 false。口径见 docs/metrics.md 第 7 节。
//
// pressed 不包含本次按下的 key；调用方负责传入「按键之前」的按下集合。
func ComboName(pressed []uint16, key uint16) (name string, ok bool) {
	// 主键本身是修饰键时，永远不构成组合。
	if isModifier(key) {
		return "", false
	}

	var ctrl, shift, alt, win bool
	for _, p := range pressed {
		switch normModifier(p) {
		case VKControl:
			ctrl = true
		case VKShift:
			shift = true
		case VKMenu:
			alt = true
		case VKLWin:
			win = true
		}
	}

	hasStrongModifier := ctrl || alt || win
	if !hasStrongModifier && !shift {
		return "", false
	}

	// 只有 Shift 时，仅当主键不是字符键才算组合：
	// Shift+A / Shift+1 属于正常输入，混进来会污染数据。
	if !hasStrongModifier && isCharacterKey(key) {
		return "", false
	}

	parts := make([]string, 0, 5)
	if ctrl {
		parts = append(parts, "Ctrl")
	}
	if shift {
		parts = append(parts, "Shift")
	}
	if alt {
		parts = append(parts, "Alt")
	}
	if win {
		parts = append(parts, "Win")
	}
	parts = append(parts, KeyName(key))

	return strings.Join(parts, "+"), true
}

// isModifier 判断键码是否为修饰键（含左右区分的形式）。
func isModifier(vk uint16) bool {
	return normModifier(vk) != 0
}

// normModifier 把左右修饰键合并成统一的键码；非修饰键返回 0。
func normModifier(vk uint16) uint16 {
	switch vk {
	case VKControl, VKLControl, VKRControl:
		return VKControl
	case VKShift, VKLShift, VKRShift:
		return VKShift
	case VKMenu, VKLMenu, VKRMenu:
		return VKMenu
	case VKLWin, VKRWin:
		return VKLWin
	}
	return 0
}

// isCharacterKey 判断主键是否会产生输入字符：字母、主键盘数字、主键盘标点。
func isCharacterKey(vk uint16) bool {
	if vk >= 0x41 && vk <= 0x5A { // A-Z
		return true
	}
	if vk >= 0x30 && vk <= 0x39 { // 0-9
		return true
	}
	_, ok := oemKeys[vk]
	return ok
}
