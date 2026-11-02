package stats

import (
	"math"
	"sort"
)

// KeyCount 是单个按键的统计行。
type KeyCount struct {
	VK    uint16
	Count int64
}

// ComboCount 是单个组合键的统计行。
type ComboCount struct {
	Combo string
	Count int64
}

// HourCount 是某个小时的按键次数，用于时段分布聚合。
type HourCount struct {
	Hour  int
	Count int64
}

// TopKeys 返回按次数降序的前 n 个键。
//
// 次数相同时按 vk 升序，保证同一份数据每次得到完全相同的顺序——
// 界面上的排行榜如果顺序抖动，用户会以为数据在乱变。
func TopKeys(counts map[uint16]int64, n int) []KeyCount {
	if n <= 0 || len(counts) == 0 {
		return nil
	}
	out := make([]KeyCount, 0, len(counts))
	for vk, c := range counts {
		out = append(out, KeyCount{VK: vk, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].VK < out[j].VK
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// TopCombos 返回按次数降序的前 n 个组合键；次数相同按组合名升序。
func TopCombos(counts map[string]int64, n int) []ComboCount {
	if n <= 0 || len(counts) == 0 {
		return nil
	}
	out := make([]ComboCount, 0, len(counts))
	for name, c := range counts {
		out = append(out, ComboCount{Combo: name, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Combo < out[j].Combo
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// Percent 返回 part 占 total 的整数百分比（四舍五入）。
// 分母非正时返回 0，避免调用方每次都要判空。
func Percent(part, total int64) int {
	if total <= 0 {
		return 0
	}
	return int(math.Round(float64(part) * 100 / float64(total)))
}

// HourlyDistribution 把多行小时计数合并成固定的 24 个桶。
// 非法小时（负数或 >23）直接忽略，不让脏数据 panic。
func HourlyDistribution(rows []HourCount) [24]int64 {
	var out [24]int64
	for _, r := range rows {
		if r.Hour < 0 || r.Hour > 23 {
			continue
		}
		out[r.Hour] += r.Count
	}
	return out
}

// DailyAverage 计算日均按键，向下取整。
func DailyAverage(total int64, days int) int64 {
	if days <= 0 {
		return 0
	}
	return total / int64(days)
}

// Peak 返回单日按键数最高的一天。
//
// 并列时取最近的一天：日期是 YYYY-MM-DD，字符串比较即时间比较。
func Peak(perDay map[string]int64) (day string, count int64, ok bool) {
	for d, c := range perDay {
		switch {
		case !ok, c > count, c == count && d > day:
			day, count, ok = d, c, true
		}
	}
	return day, count, ok
}
