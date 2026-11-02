package stats

import (
	"testing"
	"time"
)

func TestActivityTracker(t *testing.T) {
	base := time.Date(2026, 11, 2, 9, 0, 0, 0, time.Local)

	t.Run("第一次按键不产生活跃时长", func(t *testing.T) {
		var a ActivityTracker
		if got := a.Press(base); len(got) != 0 {
			t.Fatalf("首次按键得到 %v，期望空", got)
		}
	})

	t.Run("间隔内的相邻按键累加间隔", func(t *testing.T) {
		var a ActivityTracker
		a.Press(base)
		got := a.Press(base.Add(10 * time.Second))
		want := []DaySeconds{{Day: "2026-11-02", Sec: 10}}
		assertDaySeconds(t, got, want)
	})

	t.Run("刚好 60 秒算连续", func(t *testing.T) {
		var a ActivityTracker
		a.Press(base)
		got := a.Press(base.Add(60 * time.Second))
		assertDaySeconds(t, got, []DaySeconds{{Day: "2026-11-02", Sec: 60}})
	})

	t.Run("超过 60 秒断开会话", func(t *testing.T) {
		var a ActivityTracker
		a.Press(base)
		if got := a.Press(base.Add(61 * time.Second)); len(got) != 0 {
			t.Fatalf("超过 60 秒应断开，得到 %v", got)
		}
	})

	t.Run("跨午夜按天拆分", func(t *testing.T) {
		var a ActivityTracker
		prev := time.Date(2026, 11, 2, 23, 59, 50, 0, time.Local)
		next := time.Date(2026, 11, 3, 0, 0, 5, 0, time.Local) // 间隔 15 秒

		a.Press(prev)
		got := a.Press(next)
		assertDaySeconds(t, got, []DaySeconds{
			{Day: "2026-11-02", Sec: 10},
			{Day: "2026-11-03", Sec: 5},
		})
	})

	t.Run("时钟回拨不累加负时长", func(t *testing.T) {
		var a ActivityTracker
		a.Press(base)
		if got := a.Press(base.Add(-5 * time.Second)); len(got) != 0 {
			t.Fatalf("时钟回拨应得到空，实际 %v", got)
		}
	})

	t.Run("Reset 后重新开始会话", func(t *testing.T) {
		var a ActivityTracker
		a.Press(base)
		a.Reset()
		if got := a.Press(base.Add(30 * time.Second)); len(got) != 0 {
			t.Fatalf("Reset 后应按首次按键处理，得到 %v", got)
		}
	})
}

func TestTotalSeconds(t *testing.T) {
	got := TotalSeconds([]DaySeconds{
		{Day: "2026-11-02", Sec: 10},
		{Day: "2026-11-03", Sec: 5},
		{Day: "2026-11-02", Sec: 7},
	})
	if got != 22 {
		t.Fatalf("TotalSeconds = %d，期望 22", got)
	}

	if got := TotalSeconds(nil); got != 0 {
		t.Fatalf("空切片 = %d，期望 0", got)
	}
}

func assertDaySeconds(t *testing.T, got, want []DaySeconds) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("得到 %v，期望 %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("第 %d 项 = %+v，期望 %+v（完整得到 %v）", i, got[i], want[i], got)
		}
	}
}
