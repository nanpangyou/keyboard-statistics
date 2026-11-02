package stats

import "testing"

func TestNormalizeVK(t *testing.T) {
	tests := []struct {
		in   uint16
		want uint16
		desc string
	}{
		{VKLControl, VKControl, "左 Ctrl 归一"},
		{VKRControl, VKControl, "右 Ctrl 归一"},
		{VKLShift, VKShift, "左 Shift 归一"},
		{VKRShift, VKShift, "右 Shift 归一"},
		{VKMenu, VKMenu, "通用 Alt 不变"},
		{VKLMenu, VKMenu, "左 Alt 归一"},
		{VKRMenu, VKMenu, "右 Alt 归一"},
		{VKLWin, VKLWin, "左 Win 保留（左右是两个不同键）"},
		{VKRWin, VKRWin, "右 Win 保留"},
		{VKA, VKA, "普通键不变"},
		{VKEnter, VKEnter, "回车不变"},
	}
	for _, tt := range tests {
		if got := NormalizeVK(tt.in); got != tt.want {
			t.Errorf("%s: NormalizeVK(0x%X) = 0x%X，期望 0x%X", tt.desc, tt.in, got, tt.want)
		}
	}
}

func TestNormalizedKeyNamesAreStable(t *testing.T) {
	// 归一化后，左右键必须给出同一个显示名，否则统计表里会出现两行「Shift」
	for _, pair := range [][2]uint16{
		{VKLControl, VKRControl},
		{VKLShift, VKRShift},
		{VKLMenu, VKRMenu},
		{VKControl, VKLControl},
	} {
		l := KeyName(NormalizeVK(pair[0]))
		r := KeyName(NormalizeVK(pair[1]))
		if l != r {
			t.Errorf("0x%X 与 0x%X 归一后名字不同：%q vs %q", pair[0], pair[1], l, r)
		}
	}
}
