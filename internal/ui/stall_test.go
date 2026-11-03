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

// 真的恢复（安静了超过一个 repeat）之后要重新武装：再卡住时立刻写。
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
	if !tr.Observe(now.Add(40*time.Second), false) {
		t.Fatal("恢复很久后再次卡住，应当立刻写栈")
	}
}

// 探针会抖（成功一次又失败）。按「这次卡顿开始」节流的话，一次抖动就把闸门
// 重新打开，实测能刷成每 4 秒一份 36KB 的 goroutine 栈，日志很快被撑爆。
func TestStallTrackerFlappingDoesNotFlood(t *testing.T) {
	tr := newStallTracker(30 * time.Second)
	t0 := time.Unix(100, 0)

	if !tr.Observe(t0, false) {
		t.Fatal("第一次卡住没有要求写栈")
	}
	if tr.Observe(t0.Add(2*time.Second), true) {
		t.Fatal("恢复的那一下不该写栈")
	}
	if tr.Observe(t0.Add(4*time.Second), false) {
		t.Fatal("距上次写栈只有 4 秒，抖动不该再写一份")
	}
	if tr.Observe(t0.Add(6*time.Second), true) {
		t.Fatal("恢复的那一下不该写栈")
	}
	if !tr.Observe(t0.Add(31*time.Second), false) {
		t.Fatal("过了 repeat 就该再写一份，否则看不到卡顿的时间线")
	}
}

// 日志里要能看出「已经卡了多久、这是第几次报告」，光写 0s 等于没说。
func TestStallTrackerReportsDurationAndDumpCount(t *testing.T) {
	tr := newStallTracker(30 * time.Second)
	start := time.Unix(100, 0)

	tr.Observe(start, false)
	if got := tr.StalledFor(start.Add(7 * time.Second)); got != 7*time.Second {
		t.Fatalf("StalledFor = %v，期望 7s", got)
	}
	if got := tr.Dumps(); got != 1 {
		t.Fatalf("报告次数 = %d，期望 1", got)
	}
	tr.Observe(start.Add(30*time.Second), false)
	if got := tr.Dumps(); got != 2 {
		t.Fatalf("报告次数 = %d，期望 2", got)
	}
}
