package config

import (
	"errors"
	"testing"
)

func TestGetFlagDefaultsToFalse(t *testing.T) {
	s := newMemStore()
	if GetFlag(s, FlagTrayHintShown) {
		t.Error("没设过的标记应为 false")
	}
}

func TestSetFlagThenGet(t *testing.T) {
	s := newMemStore()
	if err := SetFlag(s, FlagTrayHintShown, true); err != nil {
		t.Fatalf("SetFlag: %v", err)
	}
	if !GetFlag(s, FlagTrayHintShown) {
		t.Error("设过之后应读到 true")
	}
	// 确认真的写进了设置项
	if v, ok, _ := s.Get(FlagTrayHintShown); !ok || v == "" {
		t.Errorf("设置项没写进去: %q ok=%v", v, ok)
	}

	if err := SetFlag(s, FlagTrayHintShown, false); err != nil {
		t.Fatalf("SetFlag(false): %v", err)
	}
	if GetFlag(s, FlagTrayHintShown) {
		t.Error("清掉之后应读到 false")
	}
}

func TestGetFlagParsesHandEditedValues(t *testing.T) {
	// 用户可能手改注册表，认得出几种常见写法；认不出来的当 false（宁可多说一次）
	cases := []struct {
		value string
		want  bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"yes", true},
		{"0", false},
		{"false", false},
		{"", false},
		{"随便写的", true}, // 非空即视为已设过：这条标记只关心「有没有」
	}
	for _, c := range cases {
		s := newMemStore()
		s.values[FlagTrayHintShown] = c.value
		if got := GetFlag(s, FlagTrayHintShown); got != c.want {
			t.Errorf("GetFlag(%q) = %v，期望 %v", c.value, got, c.want)
		}
	}
}

func TestFlagNilStoreIsSafe(t *testing.T) {
	if GetFlag(nil, FlagTrayHintShown) {
		t.Error("store 为 nil 时应返回 false")
	}
	if err := SetFlag(nil, FlagTrayHintShown, true); err == nil {
		t.Error("store 为 nil 时写入应报错")
	}
}

func TestGetFlagSwallowsReadError(t *testing.T) {
	s := newMemStore()
	s.err = errors.New("注册表坏了")
	if GetFlag(s, FlagTrayHintShown) {
		t.Error("读失败时应返回 false，不要影响启动")
	}
}
