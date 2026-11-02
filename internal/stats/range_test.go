package stats

import (
	"testing"
	"time"
)

func TestRange(t *testing.T) {
	// 固定一个本地时刻，避免测试随机器时区/当前时间漂移
	now := time.Date(2026, 11, 2, 15, 30, 0, 0, time.Local)

	tests := []struct {
		name     string
		scope    Scope
		firstDay string
		wantFrom string
		wantTo   string
	}{
		{"今日", ScopeToday, "", "2026-11-02", "2026-11-02"},
		{"近 7 天含今天", Scope7Days, "", "2026-10-27", "2026-11-02"},
		{"近 30 天含今天", Scope30Days, "", "2026-10-04", "2026-11-02"},
		{"全部有历史数据", ScopeAll, "2026-09-15", "2026-09-15", "2026-11-02"},
		{"全部但一条数据都没有", ScopeAll, "", "2026-11-02", "2026-11-02"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to := Range(now, tt.scope, tt.firstDay)
			if from != tt.wantFrom || to != tt.wantTo {
				t.Fatalf("Range = (%q, %q)，期望 (%q, %q)", from, to, tt.wantFrom, tt.wantTo)
			}
		})
	}
}

func TestRangeCrossMonthAndYear(t *testing.T) {
	now := time.Date(2026, 1, 3, 9, 0, 0, 0, time.Local)
	from, to := Range(now, Scope7Days, "")
	// 1 月 3 日往前 6 天会跨年到 12 月 28 日
	if from != "2025-12-28" || to != "2026-01-03" {
		t.Fatalf("跨年 Range = (%q, %q)，期望 (2025-12-28, 2026-01-03)", from, to)
	}
}

func TestDaysInclusive(t *testing.T) {
	tests := []struct {
		name     string
		from, to string
		want     int
	}{
		{"同一天算 1 天", "2026-11-02", "2026-11-02", 1},
		{"七天", "2026-10-27", "2026-11-02", 7},
		{"跨月", "2026-10-31", "2026-11-02", 3},
		{"顺序颠倒返回 0", "2026-11-02", "2026-10-27", 0},
		{"非法日期返回 0", "not-a-date", "2026-11-02", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DaysInclusive(tt.from, tt.to); got != tt.want {
				t.Fatalf("DaysInclusive(%q, %q) = %d，期望 %d", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestScopeLabels(t *testing.T) {
	// 界面上会直接显示这些标签，改文案要同步改测试，避免手滑
	tests := map[Scope]string{
		ScopeToday:  "今日",
		Scope7Days:  "近 7 天",
		Scope30Days: "近 30 天",
		ScopeAll:    "全部",
	}
	for scope, want := range tests {
		if got := scope.Label(); got != want {
			t.Errorf("Scope(%d).Label() = %q，期望 %q", scope, got, want)
		}
	}
}
