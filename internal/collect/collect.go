// Package collect 是采集的状态机：把「按下/松开」原始事件折算成可落库的增量。
//
// 这里刻意不含任何 Windows API，钩子只负责把事件转进来，所以长按去重、
// 组合键判定、活跃时长这些最容易出错的规则都能用普通单元测试覆盖。
package collect

import (
	"sync"
	"time"

	"keyboard-statistics/internal/stats"
	"keyboard-statistics/internal/store"
)

// 时间格式，只在内部用于「首次/最后按键」的显示。
const clockLayout = "15:04:05"

type keyBucket struct {
	Day  string
	Hour int
	VK   uint16
}

type comboBucket struct {
	Day   string
	Hour  int
	Combo string
}

// ActivityDelta 是某天的活跃增量与按键时间边界。
type ActivityDelta struct {
	Day        string
	Sec        int64
	FirstPress string
	LastPress  string
}

// Batch 是一次 flush 产生的全部增量。
type Batch struct {
	Keys     []store.KeyDelta
	Combos   []store.ComboDelta
	Activity []ActivityDelta
}

// Collector 在内存中累加计数，由后台定时或按条数 flush 到存储层。
type Collector struct {
	mu   sync.Mutex
	now  func() time.Time
	keys map[keyBucket]int64
	comb map[comboBucket]int64

	// pressed 记录当前按住的键，用来识别长按重复与组合键
	pressed map[uint16]struct{}

	activity    stats.ActivityTracker
	activitySec map[string]int64
	firstPress  map[string]string
	lastPress   map[string]string
}

// New 创建一个采集器。now 传 nil 时使用 time.Now。
func New(now func() time.Time) *Collector {
	if now == nil {
		now = time.Now
	}
	return &Collector{
		now:         now,
		keys:        make(map[keyBucket]int64),
		comb:        make(map[comboBucket]int64),
		pressed:     make(map[uint16]struct{}),
		activitySec: make(map[string]int64),
		firstPress:  make(map[string]string),
		lastPress:   make(map[string]string),
	}
}

// KeyDown 记录一次按下。长按产生的重复事件会被自动忽略，只计一次。
func (c *Collector) KeyDown(vk uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 钩子上报的是左右分开的键码（VK_LSHIFT / VK_RSHIFT），统计口径要求合并。
	// 在入口统一归一化，后面的长按去重、组合判定、分桶才不会各自处理一套。
	vk = stats.NormalizeVK(vk)

	if _, held := c.pressed[vk]; held {
		return // 系统长按会连发 KEYDOWN，这里必须吃掉
	}

	now := c.now()
	day, hour := now.Format(stats.DayLayout), now.Hour()

	// 组合判定必须基于「按下本键之前」的修饰键状态，所以先算再记
	if name, ok := stats.ComboName(heldKeys(c.pressed), vk); ok {
		c.comb[comboBucket{Day: day, Hour: hour, Combo: name}]++
	}

	c.pressed[vk] = struct{}{}
	c.keys[keyBucket{Day: day, Hour: hour, VK: vk}]++

	for _, ds := range c.activity.Press(now) {
		c.activitySec[ds.Day] += ds.Sec
	}

	ts := now.Format(clockLayout)
	if cur, ok := c.firstPress[day]; !ok || ts < cur {
		c.firstPress[day] = ts
	}
	if ts > c.lastPress[day] {
		c.lastPress[day] = ts
	}
}

// KeyUp 记录一次松开，把它从「按住的键」里移除。
func (c *Collector) KeyUp(vk uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 必须和 KeyDown 用同一份归一化结果，否则左右键码会永远留在「按住」集合里
	delete(c.pressed, stats.NormalizeVK(vk))
}

// Pending 返回当前待写入的桶数量，供后台判断是否需要提前 flush。
func (c *Collector) Pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.keys) + len(c.comb)
}

// Flush 取出并清空全部增量。返回的切片顺序不保证。
func (c *Collector) Flush() Batch {
	c.mu.Lock()
	defer c.mu.Unlock()

	b := Batch{}
	for k, v := range c.keys {
		b.Keys = append(b.Keys, store.KeyDelta{Day: k.Day, Hour: k.Hour, VK: k.VK, Count: v})
	}
	for k, v := range c.comb {
		b.Combos = append(b.Combos, store.ComboDelta{Day: k.Day, Hour: k.Hour, Combo: k.Combo, Count: v})
	}

	// 活跃时长按天汇总；即使某天只有首次按键（0 秒）也要写，否则首次按键时间会丢
	days := make(map[string]struct{}, len(c.activitySec)+len(c.firstPress))
	for day := range c.activitySec {
		days[day] = struct{}{}
	}
	for day := range c.firstPress {
		days[day] = struct{}{}
	}
	for day := range days {
		b.Activity = append(b.Activity, ActivityDelta{
			Day:        day,
			Sec:        c.activitySec[day],
			FirstPress: c.firstPress[day],
			LastPress:  c.lastPress[day],
		})
	}

	c.keys = make(map[keyBucket]int64)
	c.comb = make(map[comboBucket]int64)
	c.activitySec = make(map[string]int64)
	c.firstPress = make(map[string]string)
	c.lastPress = make(map[string]string)

	return b
}

// Reset 清空「按住的键」与会话起点。
// 钩子重装、从暂停恢复时必须调用，否则会把重启前的修饰键状态带进来。
func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pressed = make(map[uint16]struct{})
	c.activity.Reset()
}

// heldKeys 把按下集合摊平成切片，交给 stats.ComboName 判定。
func heldKeys(pressed map[uint16]struct{}) []uint16 {
	out := make([]uint16, 0, len(pressed))
	for vk := range pressed {
		out = append(out, vk)
	}
	return out
}
