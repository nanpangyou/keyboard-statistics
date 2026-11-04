package ui

import (
	"testing"
	"time"
)

// 界面刷新节奏是给用户看的「实时感」，不是内部实现细节：
// 2 秒一次时手指离开键盘到数字跳动之间能明显感到延迟，用户会以为程序没在统计。
// 这条测试把「不超过半秒」钉住，免得以后又被人以「省一次聚合」为由改回去。
func TestRefreshIntervalIsSnappy(t *testing.T) {
	if refreshInterval <= 0 {
		t.Fatalf("refreshInterval = %v，刷新间隔必须是正数", refreshInterval)
	}
	if refreshInterval > 500*time.Millisecond {
		t.Fatalf("refreshInterval = %v，超过 500ms 就失去「实时」的观感了", refreshInterval)
	}
}

// 刷新再快也不能比看门狗的探针更频繁：探针本身就是一次跨线程
// SendMessageTimeout，探针间隔若被顺手一起改小，卡顿判定会跟着变敏感。
func TestWatchdogProbeStaysCoarse(t *testing.T) {
	if stallProbeInterval < time.Second {
		t.Fatalf("stallProbeInterval = %v，探针间隔不该低于 1 秒", stallProbeInterval)
	}
	if stallProbeInterval < refreshInterval {
		t.Fatalf("探针间隔 %v 比刷新间隔 %v 还短，看门狗会跟着界面刷新一起忙",
			stallProbeInterval, refreshInterval)
	}
}
