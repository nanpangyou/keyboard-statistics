package ui

import (
	"testing"
	"time"
)

// 探针正常时不写栈：写一次就是几百 KB 的文本，不能变成常态。
func TestStallTrackerQuietWhenResponding(t *testing.T) {
	tr := newStallTracker(30 * time.Second)
	now := time.Unix(0, 0)

	for i := 0; i < 10; i++ {
		now = now.Add(2 * time.Second)
		if tr.Observe(now, true) {
			t.Fatalf("第 %d 次探针正常，却要求写栈", i)
		}
	}
	if got := tr.StalledFor(now); got != 0 {
		t.Fatalf("StalledFor = %v，期望 0", got)
	}
}

// 第一次发现卡住要立刻写：晚一步可能进程就被用户结束了，那份栈是唯一的证据。
func TestStallTrackerDumpsOnFirstStall(t *testing.T) {
	tr := newStallTracker(30 * time.Second)
	now := time.Unix(100, 0)

	if !tr.Observe(now, false) {
		t.Fatal("第一次卡住没有要求写栈")
	}
}

// 卡了很久时按间隔重复写，能看出卡了多久、期间有没有换地方卡。
func TestStallTrackerRepeatsWhileStalled(t *testing.T) {
	tr := newStallTracker(30 * time.Second)
	now := time.Unix(100, 0)

	if !tr.Observe(now, false) {
		t.Fatal("第一次卡住没有要求写栈")
	}
	if tr.Observe(now.Add(10*time.Second), false) {
		t.Fatal("间隔没到就重复写栈")
	}
	if !tr.Observe(now.Add(30*time.Second), false) {
		t.Fatal("到了重复间隔仍不写栈")
	}
	if tr.Observe(now.Add(59*time.Second), false) {
		t.Fatal("第二次写的间隔没到就重复写")
	}
	if !tr.Observe(now.Add(60*time.Second), false) {
		t.Fatal("第二次重复间隔到了仍不写")
	}
}

// 恢复之后要重新武装：下一次卡住必须当成「第一次」立刻写。
func TestStallTrackerRearmsAfterRecovery(t *testing.T) {
	tr := newStallTracker(30 * time.Second)
	now := time.Unix(100, 0)

	if !tr.Observe(now, false) {
		t.Fatal("第一次卡住没有要求写栈")
	}
	if tr.Observe(now.Add(2*time.Second), true) {
		t.Fatal("恢复正常时不该写栈")
	}
	if got := tr.StalledFor(now.Add(2 * time.Second)); got != 0 {
		t.Fatalf("恢复后 StalledFor = %v，期望 0", got)
	}
	if !tr.Observe(now.Add(4*time.Second), false) {
		t.Fatal("恢复后再次卡住，应当重新当成第一次立刻写栈")
	}
}

// StalledFor 是给日志用的，要报出「已经卡了多久」。
func TestStallTrackerReportsStalledDuration(t *testing.T) {
	tr := newStallTracker(30 * time.Second)
	start := time.Unix(100, 0)

	tr.Observe(start, false)
	if got := tr.StalledFor(start.Add(7 * time.Second)); got != 7*time.Second {
		t.Fatalf("StalledFor = %v，期望 7s", got)
	}
}
