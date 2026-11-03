package collect

import (
	"testing"
	"time"
)

// fixedClock 让测试可以自由拨动时间，且不依赖 time.Sleep。
type fixedClock struct {
	t time.Time
}

func (c *fixedClock) now() time.Time          { return c.t }
func (c *fixedClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestCollector(t *testing.T) (*Collector, *fixedClock) {
	t.Helper()
	clock := &fixedClock{t: time.Date(2026, 11, 2, 9, 0, 0, 0, time.Local)}
	return New(clock.now), clock
}

func keyCounts(b Batch) map[uint16]int64 {
	out := make(map[uint16]int64)
	for _, d := range b.Keys {
		out[d.VK] += d.Count
	}
	return out
}

func comboCounts(b Batch) map[string]int64 {
	out := make(map[string]int64)
	for _, d := range b.Combos {
		out[d.Combo] += d.Count
	}
	return out
}

func TestLongPressCountsOnce(t *testing.T) {
	c, _ := newTestCollector(t)

	// 长按时系统会连续发 KEYDOWN，中间没有任何 KEYUP
	for i := 0; i < 5; i++ {
		c.KeyDown(0x41) // A
	}

	got := keyCounts(c.Flush())
	if got[0x41] != 1 {
		t.Fatalf("长按 A 计了 %d 次，期望 1 次", got[0x41])
	}
}

func TestRepeatedPressesCount(t *testing.T) {
	c, _ := newTestCollector(t)

	for i := 0; i < 3; i++ {
		c.KeyDown(0x41)
		c.KeyUp(0x41)
	}

	if got := keyCounts(c.Flush())[0x41]; got != 3 {
		t.Fatalf("连按 3 次 A 计了 %d 次，期望 3 次", got)
	}
}

func TestComboCountsWithSingleKeys(t *testing.T) {
	c, _ := newTestCollector(t)

	// Ctrl 按住不放，连按 3 次 C —— 这就是真实的「连按 Ctrl+C」
	c.KeyDown(0x11) // Ctrl
	for i := 0; i < 3; i++ {
		c.KeyDown(0x43) // C
		c.KeyUp(0x43)
	}
	c.KeyUp(0x11)

	b := c.Flush()
	keys := keyCounts(b)
	combos := comboCounts(b)

	if keys[0x11] != 1 {
		t.Errorf("Ctrl 计了 %d 次，期望 1 次（按住不算重复）", keys[0x11])
	}
	if keys[0x43] != 3 {
		t.Errorf("C 计了 %d 次，期望 3 次", keys[0x43])
	}
	if combos["Ctrl+C"] != 3 {
		t.Errorf("Ctrl+C 计了 %d 次，期望 3 次", combos["Ctrl+C"])
	}
}

func TestComboModifierCountedAlone(t *testing.T) {
	c, _ := newTestCollector(t)

	c.KeyDown(0x11) // Ctrl
	c.KeyDown(0x56) // V
	c.KeyUp(0x56)
	c.KeyUp(0x11)

	b := c.Flush()
	if keyCounts(b)[0x11] != 1 {
		t.Error("Ctrl 也应作为单键计 1 次")
	}
	if keyCounts(b)[0x56] != 1 {
		t.Error("V 也应作为单键计 1 次")
	}
	if comboCounts(b)["Ctrl+V"] != 1 {
		t.Error("Ctrl+V 应单独计 1 次")
	}
}

func TestShiftWithLetterIsNotCombo(t *testing.T) {
	c, _ := newTestCollector(t)

	c.KeyDown(0x10) // Shift
	c.KeyDown(0x41) // A
	c.KeyUp(0x41)
	c.KeyUp(0x10)

	b := c.Flush()
	if n := len(b.Combos); n != 0 {
		t.Fatalf("Shift+A 不该产生组合键，实际产生 %d 条: %v", n, b.Combos)
	}
}

func TestHourBuckets(t *testing.T) {
	c, clock := newTestCollector(t)

	c.KeyDown(0x41)
	c.KeyUp(0x41)
	clock.advance(time.Hour) // 09:00 -> 10:00
	c.KeyDown(0x41)
	c.KeyUp(0x41)

	b := c.Flush()
	hours := map[int]int64{}
	for _, d := range b.Keys {
		hours[d.Hour] += d.Count
	}
	if hours[9] != 1 {
		t.Errorf("9 点 = %d，期望 1", hours[9])
	}
	if hours[10] != 1 {
		t.Errorf("10 点 = %d，期望 1", hours[10])
	}
}

func TestActivityCollected(t *testing.T) {
	c, clock := newTestCollector(t)

	c.KeyDown(0x41)
	c.KeyUp(0x41)
	clock.advance(10 * time.Second)
	c.KeyDown(0x41)
	c.KeyUp(0x41)

	b := c.Flush()
	if len(b.Activity) != 1 {
		t.Fatalf("活跃记录 %d 条，期望 1 条: %v", len(b.Activity), b.Activity)
	}
	a := b.Activity[0]
	if a.Day != "2026-11-02" {
		t.Errorf("日期 = %q，期望 2026-11-02", a.Day)
	}
	if a.Sec != 10 {
		t.Errorf("活跃秒数 = %d，期望 10", a.Sec)
	}
	if a.FirstPress != "09:00:00" {
		t.Errorf("首次按键 = %q，期望 09:00:00", a.FirstPress)
	}
	if a.LastPress != "09:00:10" {
		t.Errorf("最后按键 = %q，期望 09:00:10", a.LastPress)
	}
}

func TestFlushClearsBuffer(t *testing.T) {
	c, _ := newTestCollector(t)

	c.KeyDown(0x41)
	c.KeyUp(0x41)

	first := c.Flush()
	if len(first.Keys) != 1 {
		t.Fatalf("第一次 flush 应得到 1 条按键，实际 %d", len(first.Keys))
	}
	if c.Pending() != 0 {
		t.Fatalf("flush 后 Pending = %d，期望 0", c.Pending())
	}

	second := c.Flush()
	if len(second.Keys) != 0 || len(second.Combos) != 0 || len(second.Activity) != 0 {
		t.Fatalf("第二次 flush 应为空，实际 %+v", second)
	}
}

func TestPendingCountsKeys(t *testing.T) {
	c, _ := newTestCollector(t)

	if c.Pending() != 0 {
		t.Fatalf("初始 Pending = %d，期望 0", c.Pending())
	}
	c.KeyDown(0x41)
	c.KeyDown(0x42)
	if c.Pending() != 2 {
		t.Fatalf("Pending = %d，期望 2（按不同键的应计数）", c.Pending())
	}
	// 长按同一个键不增加待写条数
	for i := 0; i < 10; i++ {
		c.KeyDown(0x42)
	}
	if c.Pending() != 2 {
		t.Fatalf("长按后 Pending = %d，期望仍为 2", c.Pending())
	}
}

func TestResetDropsPressedState(t *testing.T) {
	c, _ := newTestCollector(t)

	c.KeyDown(0x11) // 假设钩子重启时 Ctrl 还按着
	c.Reset()

	// Reset 之后重新按 C，不该再判定成 Ctrl+C
	c.KeyDown(0x43)
	c.KeyUp(0x43)

	b := c.Flush()
	if n := len(b.Combos); n != 0 {
		t.Fatalf("Reset 后不该残留修饰键状态，实际产生 %d 条组合: %v", n, b.Combos)
	}
}

// Peek 是给界面实时取数用的：要能看到还没落库的增量，但不能把它取走。
func TestPeekDoesNotConsumePending(t *testing.T) {
	c, _ := newTestCollector(t)

	c.KeyDown(0x41)
	c.KeyUp(0x41)

	first := c.Peek()
	if keyCounts(first)[0x41] != 1 {
		t.Fatalf("Peek 拿到 %v，期望 A=1", keyCounts(first))
	}
	if c.Pending() != 1 {
		t.Fatalf("Peek 之后 Pending = %d，期望 1（取数不能把增量吃掉）", c.Pending())
	}

	// 再按一次，Peek 应该看到累计值；此时 Flush 仍要能拿到全部 2 次
	c.KeyDown(0x41)
	c.KeyUp(0x41)

	second := c.Peek()
	if keyCounts(second)[0x41] != 2 {
		t.Fatalf("第二次 Peek 拿到 %v，期望 A=2", keyCounts(second))
	}
	if got := keyCounts(c.Flush())[0x41]; got != 2 {
		t.Fatalf("Flush 拿到 %d，期望 2（Peek 不能影响落库）", got)
	}
}

// Peek 返回的必须是快照：事后继续按键不能篡改已经交出去的那一份。
func TestPeekReturnsIndependentCopy(t *testing.T) {
	c, _ := newTestCollector(t)

	c.KeyDown(0x41)
	c.KeyUp(0x41)
	snap := c.Peek()

	c.KeyDown(0x41)
	c.KeyUp(0x41)

	if got := keyCounts(snap)[0x41]; got != 1 {
		t.Fatalf("早先那份快照里的 A = %d，期望 1（不该被后来的按键改掉）", got)
	}
}
