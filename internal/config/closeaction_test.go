package config

import (
	"errors"
	"testing"
)

func TestParseCloseAction(t *testing.T) {
	cases := []struct {
		in   string
		want CloseAction
	}{
		{"", CloseAsk},        // 没设置过
		{"tray", CloseToTray}, //
		{"exit", CloseExit},   //
		{"TRAY", CloseToTray}, // 大小写不敏感，手改注册表不该炸
		{" exit ", CloseExit}, // 两边空白容错
		{"trayy", CloseAsk},   // 不认识的值一律当成「没选过」，宁可多问一次
		{"退出", CloseAsk},      //
	}
	for _, c := range cases {
		if got := ParseCloseAction(c.in); got != c.want {
			t.Errorf("ParseCloseAction(%q) = %v，期望 %v", c.in, got, c.want)
		}
	}
}

func TestCloseActionStringRoundTrip(t *testing.T) {
	for _, a := range []CloseAction{CloseAsk, CloseToTray, CloseExit} {
		if got := ParseCloseAction(a.String()); got != a {
			t.Errorf("%v.String() = %q，解析回来变成 %v", a, a.String(), got)
		}
	}
}

func TestCloseActionToTray(t *testing.T) {
	// 界面只关心「该不该收进托盘」；没选过（CloseAsk）时按收进托盘处理，
	// 因为真正的选择在关窗那一刻才问。
	if !CloseAsk.ToTray() {
		t.Error("没选过时应默认收进托盘（关窗时会先问）")
	}
	if !CloseToTray.ToTray() {
		t.Error("CloseToTray.ToTray() 应为真")
	}
	if CloseExit.ToTray() {
		t.Error("CloseExit.ToTray() 应为假")
	}
}

func TestGetCloseActionReadsStore(t *testing.T) {
	s := newMemStore()
	if got := GetCloseAction(s); got != CloseAsk {
		t.Errorf("空存储应返回 CloseAsk，实际 %v", got)
	}

	if err := SetCloseAction(s, CloseExit); err != nil {
		t.Fatalf("SetCloseAction: %v", err)
	}
	if got := GetCloseAction(s); got != CloseExit {
		t.Errorf("存了 exit 后读到 %v，期望 CloseExit", got)
	}
	// 确认真的落到了设置项上（而不是只写在内存里的别处）
	if v, ok, _ := s.Get(closeActionSetting); !ok || v != "exit" {
		t.Errorf("注册表值为 %q, ok=%v，期望 \"exit\"", v, ok)
	}
}

func TestCloseActionNilStoreIsSafe(t *testing.T) {
	// 拿不到设置存储（权限异常等）时不能崩，退回「没选过」
	if got := GetCloseAction(nil); got != CloseAsk {
		t.Errorf("store 为 nil 时应返回 CloseAsk，实际 %v", got)
	}
	if err := SetCloseAction(nil, CloseToTray); err == nil {
		t.Error("store 为 nil 时写入应报错，而不是假装成功")
	}
}

func TestCloseActionStoreErrorsAreSwallowedOnRead(t *testing.T) {
	// 读设置失败不阻断关窗流程：最多再问用户一次
	s := newMemStore()
	s.err = errors.New("注册表坏了")
	if got := GetCloseAction(s); got != CloseAsk {
		t.Errorf("读失败时应返回 CloseAsk，实际 %v", got)
	}
}
