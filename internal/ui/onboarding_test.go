package ui

import (
	"strings"
	"testing"
)

// 首次启动说明框的文案是「不隐藏采集」这条红线的落点：
// 用户必须在第一次启动时就知道程序已经在采集、采什么、怎么停。
// 三句话少一句都算不合格，所以用测试锁住。
func TestOnboardingTextCoversRequiredPoints(t *testing.T) {
	text := onboardingText()

	required := []struct {
		what string
		want string
	}{
		{"说明已经开始采集", "已经开始采集"},
		{"说明只记键码与次数、不记内容", "只记录每个键按了多少次"},
		{"说明不记输入内容", "不记录你输入的任何内容"},
		{"说明可以暂停", "随时可以暂停"},
		{"说明数据在本机", "数据只保存在本机"},
	}
	for _, r := range required {
		if !strings.Contains(text, r.want) {
			t.Errorf("首次启动说明缺少「%s」：应包含 %q，实际文案：\n%s", r.what, r.want, text)
		}
	}
}

// 说明框不能太啰嗦：内容超过一屏用户就不看了。
func TestOnboardingTextIsShortEnough(t *testing.T) {
	text := onboardingText()
	if lines := strings.Count(text, "\n") + 1; lines > 12 {
		t.Errorf("首次启动说明有 %d 行，超过 12 行会一屏放不下", lines)
	}
}
