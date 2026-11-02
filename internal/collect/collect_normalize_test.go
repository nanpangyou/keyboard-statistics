package collect

import (
	"testing"

	"keyboard-statistics/internal/stats"
)

// 左右修饰键在钩子里是两个不同键码（VK_LSHIFT / VK_RSHIFT 等），
// 但统计口径要求合并。这里锁住「采集时就归一化」这件事：
// 如果漏了归一化，热力图按通用键码 0x10 取数会永远显示 0，TOP 榜还会出现两行 Shift。
func TestModifierLeftAndRightCountedAsOneKey(t *testing.T) {
	c, _ := newTestCollector(t)

	c.KeyDown(stats.VKLShift)
	c.KeyUp(stats.VKLShift)
	c.KeyDown(stats.VKRShift)
	c.KeyUp(stats.VKRShift)

	got := keyCounts(c.Flush())

	if n := got[stats.VKShift]; n != 2 {
		t.Errorf("Shift 计数 = %d，期望 2（左右各一次合并）", n)
	}
	for _, vk := range []uint16{stats.VKLShift, stats.VKRShift} {
		if n := got[vk]; n != 0 {
			t.Errorf("不该再出现独立的左右键码 0x%X：计数 = %d", vk, n)
		}
	}
}

// Ctrl 也一样：两个键码必须落到同一个桶里，否则按键总数会被拆成两行。
func TestCtrlLeftAndRightShareOneBucket(t *testing.T) {
	c, _ := newTestCollector(t)

	c.KeyDown(stats.VKLControl)
	c.KeyUp(stats.VKLControl)
	c.KeyDown(stats.VKRControl)
	c.KeyUp(stats.VKRControl)

	got := keyCounts(c.Flush())
	if n := got[stats.VKControl]; n != 2 {
		t.Errorf("Ctrl 计数 = %d，期望 2", n)
	}
	if len(got) != 1 {
		t.Errorf("桶数量 = %d，期望 1（左右 Ctrl 合并后只剩一个键码）：%v", len(got), got)
	}
}

// 归一化不能影响组合键：左 Shift + 方向键仍然要算 Shift+←。
// （Shift + 字母不算组合键，那是打大写字母，见 docs/metrics.md）
func TestNormalizedModifierStillFormsCombo(t *testing.T) {
	c, _ := newTestCollector(t)

	c.KeyDown(stats.VKLShift)
	c.KeyDown(stats.VKLeft)
	c.KeyUp(stats.VKLeft)
	c.KeyUp(stats.VKLShift)

	combos := comboCounts(c.Flush())
	if got := combos["Shift+←"]; got != 1 {
		t.Errorf("Shift+← 组合计数 = %d，期望 1；实际组合=%v", got, combos)
	}
}

// 右键松开也必须能清掉归一化后的按住状态，否则下一次按键会被误判成长按重复。
func TestRightModifierKeyUpClearsHeldState(t *testing.T) {
	c, _ := newTestCollector(t)

	c.KeyDown(stats.VKRControl)
	c.KeyUp(stats.VKRShift) // 归一到 Shift，跟 Ctrl 无关，不应清掉 Ctrl
	if _, held := c.pressed[stats.VKControl]; !held {
		t.Fatal("松开 Shift 不该把按住中的 Ctrl 清掉")
	}

	c.KeyUp(stats.VKRControl)
	if _, held := c.pressed[stats.VKControl]; held {
		t.Error("松开右 Ctrl 后应清掉归一化的通用 Ctrl 按住状态")
	}
}
