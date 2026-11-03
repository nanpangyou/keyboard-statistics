package ui

import "time"

// stallTracker 决定「界面线程卡住了，要不要写一份 goroutine 栈」。
//
// 从外面看，卡住和「正常地等消息」长得一模一样（都是 CPU 0、都在 Wait），
// 所以只能由外部探针告诉我们「刚才那一下没得到回应」。这个类型只管节流：
// 卡住的瞬间写一次，之后每隔 repeat 再写一次（卡很久时能看到时间线）。
//
// 节流看的是「距上次写栈多久」，**不是**「这次卡顿开始了多久」：
// 探针会抖（刚超时紧接着又成功），按卡顿开始算的话一次抖动就把闸门重新打开，
// 实测能把日志刷成每 4 秒一份 36KB 的栈。
//
// 纯逻辑，不碰 Win32，所以能直接测。
type stallTracker struct {
	repeat time.Duration

	// 0 表示当前没卡；否则是这次卡顿开始的时间
	stalledSince time.Time
	lastDump     time.Time
	dumps        int
}

// newStallTracker 建一个节流器：同一段卡顿里，最多每 repeat 写一次栈。
func newStallTracker(repeat time.Duration) *stallTracker {
	return &stallTracker{repeat: repeat}
}

// Observe 喂一次探针结果：responded 为真表示界面线程正常处理了消息。
// 返回 true 表示「现在该写一份栈了」。
func (s *stallTracker) Observe(now time.Time, responded bool) bool {
	if responded {
		s.stalledSince = time.Time{}
		return false
	}

	if s.stalledSince.IsZero() {
		s.stalledSince = now
	}
	if !s.lastDump.IsZero() && now.Sub(s.lastDump) < s.repeat {
		return false
	}
	s.lastDump = now
	s.dumps++
	return true
}

// StalledFor 返回已经卡了多久（没卡返回 0），用于日志里写明卡顿时长。
func (s *stallTracker) StalledFor(now time.Time) time.Duration {
	if s.stalledSince.IsZero() {
		return 0
	}
	return now.Sub(s.stalledSince)
}

// Dumps 返回已经写过几份栈，日志里带上它就能看出「卡了多久、报了几次」。
func (s *stallTracker) Dumps() int { return s.dumps }
