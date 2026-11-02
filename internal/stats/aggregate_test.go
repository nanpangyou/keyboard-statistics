package stats

import "testing"

func TestTopKeys(t *testing.T) {
	tests := []struct {
		name   string
		counts map[uint16]int64
		n      int
		want   []KeyCount
	}{
		{
			name:   "按次数降序",
			counts: map[uint16]int64{VKA: 5, VKB: 30, VKC: 12},
			n:      3,
			want:   []KeyCount{{VKB, 30}, {VKC, 12}, {VKA, 5}},
		},
		{
			name:   "次数相同按 vk 升序，保证顺序稳定",
			counts: map[uint16]int64{VKC: 10, VKA: 10, VKB: 10},
			n:      3,
			want:   []KeyCount{{VKA, 10}, {VKB, 10}, {VKC, 10}},
		},
		{
			name:   "只取前 n 个",
			counts: map[uint16]int64{VKA: 100, VKB: 50, VKC: 10},
			n:      2,
			want:   []KeyCount{{VKA, 100}, {VKB, 50}},
		},
		{
			name:   "n 超过实际数量时全给",
			counts: map[uint16]int64{VKA: 3, VKB: 2},
			n:      10,
			want:   []KeyCount{{VKA, 3}, {VKB, 2}},
		},
		{
			name:   "空数据",
			counts: nil,
			n:      10,
			want:   nil,
		},
		{
			name:   "n 非正数",
			counts: map[uint16]int64{VKA: 1},
			n:      0,
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TopKeys(tt.counts, tt.n)
			assertKeyCounts(t, got, tt.want)
		})
	}
}

func TestTopCombos(t *testing.T) {
	tests := []struct {
		name   string
		counts map[string]int64
		n      int
		want   []ComboCount
	}{
		{
			name:   "按次数降序",
			counts: map[string]int64{"Ctrl+C": 7, "Ctrl+V": 20, "Alt+Tab": 3},
			n:      3,
			want:   []ComboCount{{"Ctrl+V", 20}, {"Ctrl+C", 7}, {"Alt+Tab", 3}},
		},
		{
			name:   "次数相同按名称升序",
			counts: map[string]int64{"Win+D": 5, "Alt+Tab": 5, "Ctrl+C": 5},
			n:      3,
			want:   []ComboCount{{"Alt+Tab", 5}, {"Ctrl+C", 5}, {"Win+D", 5}},
		},
		{
			name:   "空数据",
			counts: nil,
			n:      10,
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TopCombos(tt.counts, tt.n)
			if len(got) != len(tt.want) {
				t.Fatalf("长度 = %d，期望 %d（got=%v）", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("第 %d 项 = %+v，期望 %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPercent(t *testing.T) {
	tests := []struct {
		part, total int64
		want        int
	}{
		{21, 100, 21},
		{1, 3, 33},
		{2, 3, 67},
		{1, 2, 50},
		{0, 100, 0},
		{5, 5, 100},
		{0, 0, 0},       // 分母为 0 不能 panic
		{5, 0, 0},       // 数据异常时也不能 panic
		{150, 100, 150}, // 不做裁剪，交给调用方判断
	}
	for _, tt := range tests {
		if got := Percent(tt.part, tt.total); got != tt.want {
			t.Errorf("Percent(%d, %d) = %d，期望 %d", tt.part, tt.total, got, tt.want)
		}
	}
}

func TestHourlyDistribution(t *testing.T) {
	t.Run("按小时累加并补零", func(t *testing.T) {
		rows := []HourCount{
			{Hour: 0, Count: 3},
			{Hour: 0, Count: 4},
			{Hour: 9, Count: 10},
			{Hour: 23, Count: 7},
		}
		got := HourlyDistribution(rows)

		if got[0] != 7 {
			t.Errorf("0 点 = %d，期望 7", got[0])
		}
		if got[9] != 10 {
			t.Errorf("9 点 = %d，期望 10", got[9])
		}
		if got[23] != 7 {
			t.Errorf("23 点 = %d，期望 7", got[23])
		}
		if got[12] != 0 {
			t.Errorf("12 点 = %d，期望 0", got[12])
		}
	})

	t.Run("空数据返回 24 个零", func(t *testing.T) {
		got := HourlyDistribution(nil)
		for h := 0; h < 24; h++ {
			if got[h] != 0 {
				t.Fatalf("%d 点 = %d，期望 0", h, got[h])
			}
		}
	})

	t.Run("非法小时不 panic", func(t *testing.T) {
		got := HourlyDistribution([]HourCount{{Hour: -1, Count: 5}, {Hour: 99, Count: 5}})
		for h := 0; h < 24; h++ {
			if got[h] != 0 {
				t.Fatalf("%d 点 = %d，期望被忽略", h, got[h])
			}
		}
	})
}

func TestDailyAverage(t *testing.T) {
	tests := []struct {
		name  string
		total int64
		days  int
		want  int64
	}{
		{"今日一天", 100, 1, 100},
		{"七天向下取整", 100, 7, 14},
		{"正好整除", 140, 7, 20},
		{"不足一天", 3, 7, 0},
		{"无数据", 0, 30, 0},
		{"天数非法", 100, 0, 0},
		{"天数为负", 100, -5, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DailyAverage(tt.total, tt.days); got != tt.want {
				t.Fatalf("DailyAverage(%d, %d) = %d，期望 %d", tt.total, tt.days, got, tt.want)
			}
		})
	}
}

func TestPeak(t *testing.T) {
	t.Run("取最大的一天", func(t *testing.T) {
		day, count, ok := Peak(map[string]int64{
			"2026-11-01": 100,
			"2026-11-02": 150,
			"2026-11-03": 120,
		})
		if !ok || day != "2026-11-02" || count != 150 {
			t.Fatalf("得到 %q/%d/%v，期望 2026-11-02/150/true", day, count, ok)
		}
	})

	t.Run("并列时取最近的一天", func(t *testing.T) {
		day, count, ok := Peak(map[string]int64{
			"2026-11-01": 150,
			"2026-11-03": 150,
			"2026-11-02": 150,
		})
		if !ok || day != "2026-11-03" || count != 150 {
			t.Fatalf("得到 %q/%d/%v，期望 2026-11-03/150/true", day, count, ok)
		}
	})

	t.Run("空数据", func(t *testing.T) {
		_, _, ok := Peak(nil)
		if ok {
			t.Fatal("空数据时期望 ok=false")
		}
	})
}

func assertKeyCounts(t *testing.T, got, want []KeyCount) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("长度 = %d，期望 %d（got=%v）", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("第 %d 项 = %+v，期望 %+v", i, got[i], want[i])
		}
	}
}
