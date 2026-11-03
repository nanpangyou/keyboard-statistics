package service

import (
	"testing"
	"time"

	"keyboard-statistics/internal/stats"
)

// 界面每 2 秒取一次数，但落库是 30 秒一次：如果不把内存里还没落库的增量算进来，
// 用户打完一串字要等半分钟才看到数字动——这就是「要等一分钟才刷新」的真相。
func TestSnapshotIncludesUnflushedCounts(t *testing.T) {
	svc, _ := newTestService(t)

	// 故意不 FlushNow：这些计数此刻只在内存里
	svc.KeyDown(0x41) // A
	svc.KeyUp(0x41)
	svc.KeyDown(0x42) // B
	svc.KeyUp(0x42)
	svc.KeyDown(0x42)
	svc.KeyUp(0x42)

	snap, err := svc.Snapshot(stats.ScopeToday)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if snap.TotalKeys != 3 {
		t.Fatalf("总按键 = %d，期望 3（内存里的 3 次还没落库也要算）", snap.TotalKeys)
	}
	if len(snap.TopKeys) == 0 || snap.TopKeys[0].VK != 0x42 || snap.TopKeys[0].Count != 2 {
		t.Fatalf("TopKeys[0] = %+v，期望 VK=0x42 Count=2", snap.TopKeys[0])
	}
	if got := snap.KeyCounts[0x41]; got != 1 {
		t.Fatalf("KeyCounts[0x41] = %d，期望 1", got)
	}

	// 再取一次：内存增量没被取走（不像 Flush 会清空），数字必须一样
	again, err := svc.Snapshot(stats.ScopeToday)
	if err != nil {
		t.Fatalf("第二次 Snapshot: %v", err)
	}
	if again.TotalKeys != 3 {
		t.Fatalf("第二次总按键 = %d，期望仍是 3（取数不能消耗掉未落库的增量）", again.TotalKeys)
	}
}

// 落库之后内存里就空了，不能出现「数据库一份 + 内存一份」的重复计数。
func TestSnapshotDoesNotDoubleCountAfterFlush(t *testing.T) {
	svc, _ := newTestService(t)

	svc.KeyDown(0x41)
	svc.KeyUp(0x41)

	if err := svc.FlushNow(); err != nil {
		t.Fatalf("FlushNow: %v", err)
	}

	snap, err := svc.Snapshot(stats.ScopeToday)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.TotalKeys != 1 {
		t.Fatalf("落库后总按键 = %d，期望 1（不能重复计一次）", snap.TotalKeys)
	}
}

// 时段分布同样要算上内存增量，否则柱状图要等落库才动。
func TestSnapshotIncludesUnflushedHours(t *testing.T) {
	svc, clock := newTestService(t)
	clock.t = clock.t.Add(3 * time.Hour) // 12 点

	svc.KeyDown(0x41)
	svc.KeyUp(0x41)

	snap, err := svc.Snapshot(stats.ScopeToday)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if got := snap.Hourly[12]; got != 1 {
		t.Fatalf("12 点 = %d，期望 1（未落库的也要算）", got)
	}
}

// 时间范围之外的未落库增量不能被算进来：近 7 天视图不该被今天的口径污染成别的样子。
func TestSnapshotSkipsUnflushedOutsideScope(t *testing.T) {
	svc, clock := newTestService(t)

	svc.KeyDown(0x41) // 记在「今天」
	svc.KeyUp(0x41)

	// 把时钟拨到 10 天后：这份增量就落在近 7 天范围之外了
	clock.t = clock.t.AddDate(0, 0, 10)

	snap, err := svc.Snapshot(stats.Scope7Days)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.TotalKeys != 0 {
		t.Fatalf("近 7 天总按键 = %d，期望 0（10 天前那次不该算进来）", snap.TotalKeys)
	}
}
