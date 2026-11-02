package stats

import "testing"

// TestComboEligibility 固化 docs/metrics.md 第 7 节的组合键判定规则。
// pressed 是「本次按键之前」已经按住的键集合；key 是本次按下的键。
func TestComboEligibility(t *testing.T) {
	tests := []struct {
		name    string
		pressed []uint16
		key     uint16
		want    string // 期望的组合名；空字符串表示不构成组合
	}{
		// 含 Ctrl：字符键也成立
		{"Ctrl+C", []uint16{VKControl}, VKC, "Ctrl+C"},
		{"Ctrl+V", []uint16{VKControl}, VKV, "Ctrl+V"},
		{"Ctrl+Z", []uint16{VKControl}, VKZ, "Ctrl+Z"},

		// 含 Alt：微信截屏这类应⽤热键
		{"Alt+A", []uint16{VKMenu}, VKA, "Alt+A"},
		{"Alt+Tab", []uint16{VKMenu}, VKTab, "Alt+Tab"},
		{"Alt+F4", []uint16{VKMenu}, VKF4, "Alt+F4"},

		// Win 组合
		{"Win+D", []uint16{VKLWin}, VKD, "Win+D"},
		{"Win+R", []uint16{VKLWin}, VKR, "Win+R"},

		// 多修饰键，顺序固定 Ctrl→Shift→Alt→Win
		{"Ctrl+Shift+T", []uint16{VKControl, VKShift}, VKT, "Ctrl+Shift+T"},
		{"Ctrl+Alt+Delete", []uint16{VKControl, VKMenu}, VKDelete, "Ctrl+Alt+Delete"},
		{"Shift+Alt+T", []uint16{VKShift, VKMenu}, VKT, "Shift+Alt+T"},

		// 只有 Shift：非字符键成立
		{"Shift+Left", []uint16{VKShift}, VKLeft, "Shift+←"},
		{"Shift+Enter", []uint16{VKShift}, VKEnter, "Shift+回车"},
		{"Shift+Tab", []uint16{VKShift}, VKTab, "Shift+Tab"},

		// 只有 Shift + 字符键：不成立（那是打大写/符号）
		{"Shift+A", []uint16{VKShift}, VKA, ""},
		{"Shift+1", []uint16{VKShift}, VK1, ""},

		// 主键是修饰键本身：永不构成组合
		{"单按 Ctrl", nil, VKControl, ""},
		{"单按 Shift", nil, VKShift, ""},
		{"Ctrl + Shift", []uint16{VKControl}, VKShift, ""},

		// 没有任何修饰键
		{"单按 A", nil, VKA, ""},
		{"按住 A 再按 B", []uint16{VKA}, VKB, ""},

		// 左右修饰键合并（不区分 VK_LWIN / VK_RWIN）
		{"RWin+D", []uint16{VKRWin}, VKD, "Win+D"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ComboName(tt.pressed, tt.key)
			if tt.want == "" {
				if ok {
					t.Fatalf("期望不构成组合，实际得到 %q", got)
				}
				return
			}
			if !ok {
				t.Fatalf("期望组合 %q，实际判定为不构成组合", tt.want)
			}
			if got != tt.want {
				t.Fatalf("组合名 = %q，期望 %q", got, tt.want)
			}
		})
	}
}
