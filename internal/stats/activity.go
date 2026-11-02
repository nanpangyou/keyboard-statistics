package stats

import "time"

// MaxSessionGap 是「还算同一次连续使用」的最大按键间隔。
// 超过它视为用户离开后重新开始，不再计入活跃时长。
const MaxSessionGap = 60 * time.Second

// DaySeconds 是某一天新增的活跃秒数。
type DaySeconds struct {
	Day string
	Sec int64
}

// ActivityTracker 把连续的按键时间点折算成「活跃时长」。
//
// 设计上刻意不保存任何时间点：每来一次按键就当场算出增量并交给存储层累加，
// 库里只有「每天多少秒」，没有「几点几分按了键」，这才符合「不记录行为轨迹」的红线。
type ActivityTracker struct {
	last time.Time
}

// Press 记录一次按键，返回本次应累加的活跃秒数（可能为空）。
//
// 首次按键只建立会话起点，不加秒；间隔超过 MaxSessionGap 视为新会话，同样不加秒。
// 跨午夜的间隔会按天拆分，避免把今天的时长算到昨天头上。
func (a *ActivityTracker) Press(now time.Time) []DaySeconds {
	prev := a.last
	a.last = now

	if prev.IsZero() || !now.After(prev) {
		return nil
	}
	gap := now.Sub(prev)
	if gap > MaxSessionGap {
		return nil
	}

	prevDay := prev.Format(DayLayout)
	nowDay := now.Format(DayLayout)
	if prevDay == nowDay {
		return []DaySeconds{{Day: nowDay, Sec: int64(gap.Seconds())}}
	}

	// 跨天：从上次按键到次日零点算前一天，零点到本次按键算今天
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return []DaySeconds{
		{Day: prevDay, Sec: int64(midnight.Sub(prev).Seconds())},
		{Day: nowDay, Sec: int64(now.Sub(midnight).Seconds())},
	}
}

// Reset 丢弃当前会话起点，用于暂停采集或跨进程恢复后重新开始。
func (a *ActivityTracker) Reset() {
	a.last = time.Time{}
}

// TotalSeconds 汇总多天活跃秒数。
func TotalSeconds(secs []DaySeconds) int64 {
	var total int64
	for _, s := range secs {
		total += s.Sec
	}
	return total
}
