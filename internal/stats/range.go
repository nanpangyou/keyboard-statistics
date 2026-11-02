package stats

import "time"

// DayLayout 是数据库与界面上统一的日期格式。
const DayLayout = "2006-01-02"

// Scope 是界面顶部的时间范围。
type Scope int

const (
	ScopeToday Scope = iota
	Scope7Days
	Scope30Days
	ScopeAll
)

// scopes 定义顺序即界面按钮顺序，新增范围只需在这里加一项。
var scopes = []Scope{ScopeToday, Scope7Days, Scope30Days, ScopeAll}

// AllScopes 返回界面上要展示的时间范围按钮顺序。
func AllScopes() []Scope { return scopes }

// Label 是界面直接显示的中文标签。
func (s Scope) Label() string {
	switch s {
	case Scope7Days:
		return "近 7 天"
	case Scope30Days:
		return "近 30 天"
	case ScopeAll:
		return "全部"
	default:
		return "今日"
	}
}

// Range 计算时间范围对应的起止日期（含首尾两天）。
//
// 范围按自然日算：近 7 天是「今天 + 前 6 天」，共 7 个自然日，
// 这样日均的分母才是用户直觉里的 7。
func Range(now time.Time, scope Scope, firstDay string) (from, to string) {
	to = now.Format(DayLayout)
	switch scope {
	case Scope7Days:
		from = now.AddDate(0, 0, -6).Format(DayLayout)
	case Scope30Days:
		from = now.AddDate(0, 0, -29).Format(DayLayout)
	case ScopeAll:
		if firstDay != "" {
			from = firstDay
		} else {
			from = to
		}
	default: // ScopeToday 及未知值都按今日处理
		from = to
	}
	return from, to
}

// DaysInclusive 返回闭区间内的自然日天数，用于计算日均。
// 日期非法或区间颠倒时返回 0，调用方据此显示「—」。
func DaysInclusive(from, to string) int {
	f, err := time.Parse(DayLayout, from)
	if err != nil {
		return 0
	}
	t, err := time.Parse(DayLayout, to)
	if err != nil {
		return 0
	}
	if t.Before(f) {
		return 0
	}
	return int(t.Sub(f).Hours()/24) + 1
}
