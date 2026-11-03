package ui

import "time"

// stallTracker 决定「界面线程卡住了，要不要写一份 goroutine 栈」。
//
// 从外面看，卡住和「正常地等消息」长得一模一样（都是 CPU 0、都在 Wait），
// 所以只能由外部探针告诉我们「刚才那一下没得到回应」。这个类型只管节流：
// 卡住的瞬间写一次，之后每隔 repeat 再写一次（卡很久时能看到时间线），
// 一旦恢复就重新武装。
//
// 纯逻辑，不碰 Win32，所以能直接测。
type stallTracker struct {
	repeat time.Duration

	// 0 表示当前没卡；否则是这次卡顿开始的时间
	stalledSince time.Time
	lastDump     time.Time
}

// newStallTracker 建一个节流器：同一次卡顿里，最多每 repeat 写一次栈。
func newStallTracker(repeat time.Duration) *stallTracker {
	return &stallTracker{repeat: repeat}
}

// Observe 喂一次探针结果：responded 为真表示界面线程正常处理了消息。
// 返回 true 表示「现在该写一份栈了」。
func (s *stallTracker) Observe(now time.Time, responded bool) bool {
	if responded {
		s.stalledSince = time.Time{}
		s.lastDump = time.Time{}
		return false
	}

	if s.stalledSince.IsZero() {
		// 刚开始卡：立刻写一次，这次最有价值
		s.stalledSince = now
		s.lastDump = now
		return true
	}
	if now.Sub(s.lastDump) >= s.repeat {
		s.lastDump = now
		return true
	}
	return false
}

// StalledFor 返回已经卡了多久（没卡返回 0），用于日志里写明卡顿时长。
func (s *stallTracker) StalledFor(now time.Time) time.Duration {
	if s.stalledSince.IsZero() {
		return 0
	}
	return now.Sub(s.stalledSince)
}
