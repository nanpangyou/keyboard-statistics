package store

import (
	"path/filepath"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
	return s
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatalf("第一次 Open: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("第一次 Close: %v", err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("第二次 Open 应该能复用已有库: %v", err)
	}
	if err := s2.Close(); err != nil {
		t.Fatalf("第二次 Close: %v", err)
	}
}

func TestAddKeyCountsAccumulates(t *testing.T) {
	s := newStore(t)

	delta := []KeyDelta{
		{Day: "2026-11-02", Hour: 9, VK: 0x41, Count: 3},
		{Day: "2026-11-02", Hour: 9, VK: 0x41, Count: 4},
		{Day: "2026-11-02", Hour: 9, VK: 0x42, Count: 2},
	}
	if err := s.AddKeyCounts(delta); err != nil {
		t.Fatalf("AddKeyCounts: %v", err)
	}

	got, err := s.KeyCounts("2026-11-02", "2026-11-02")
	if err != nil {
		t.Fatalf("KeyCounts: %v", err)
	}
	if got[0x41] != 7 {
		t.Errorf("A 键 = %d，期望 7（3+4 应累加）", got[0x41])
	}
	if got[0x42] != 2 {
		t.Errorf("B 键 = %d，期望 2", got[0x42])
	}
}

func TestKeyCountsRespectsRange(t *testing.T) {
	s := newStore(t)
	mustAddKeys(t, s,
		KeyDelta{Day: "2026-10-01", Hour: 9, VK: 0x41, Count: 100},
		KeyDelta{Day: "2026-10-02", Hour: 9, VK: 0x41, Count: 20},
		KeyDelta{Day: "2026-10-03", Hour: 9, VK: 0x42, Count: 30},
	)

	got, err := s.KeyCounts("2026-10-02", "2026-10-03")
	if err != nil {
		t.Fatalf("KeyCounts: %v", err)
	}
	if got[0x41] != 20 {
		t.Errorf("A = %d，期望 20（10-01 的 100 次不该被算进范围）", got[0x41])
	}
	if got[0x42] != 30 {
		t.Errorf("B = %d，期望 30", got[0x42])
	}
}

func TestComboCounts(t *testing.T) {
	s := newStore(t)
	if err := s.AddComboCounts([]ComboDelta{
		{Day: "2026-11-02", Hour: 10, Combo: "Ctrl+C", Count: 2},
		{Day: "2026-11-02", Hour: 10, Combo: "Ctrl+C", Count: 3},
		{Day: "2026-11-02", Hour: 11, Combo: "Alt+Tab", Count: 1},
	}); err != nil {
		t.Fatalf("AddComboCounts: %v", err)
	}

	got, err := s.ComboCounts("2026-11-02", "2026-11-02")
	if err != nil {
		t.Fatalf("ComboCounts: %v", err)
	}
	if got["Ctrl+C"] != 5 {
		t.Errorf("Ctrl+C = %d，期望 5", got["Ctrl+C"])
	}
	if got["Alt+Tab"] != 1 {
		t.Errorf("Alt+Tab = %d，期望 1", got["Alt+Tab"])
	}
}

func TestHourCounts(t *testing.T) {
	s := newStore(t)
	mustAddKeys(t, s,
		KeyDelta{Day: "2026-11-01", Hour: 9, VK: 0x41, Count: 5},
		KeyDelta{Day: "2026-11-02", Hour: 9, VK: 0x42, Count: 7},
		KeyDelta{Day: "2026-11-02", Hour: 14, VK: 0x41, Count: 2},
	)

	rows, err := s.HourCounts("2026-11-01", "2026-11-02")
	if err != nil {
		t.Fatalf("HourCounts: %v", err)
	}

	byHour := map[int]int64{}
	for _, r := range rows {
		byHour[r.Hour] += r.Count
	}
	if byHour[9] != 12 {
		t.Errorf("9 点 = %d，期望 12（跨天同小时应合并）", byHour[9])
	}
	if byHour[14] != 2 {
		t.Errorf("14 点 = %d，期望 2", byHour[14])
	}
}

func TestDailyTotals(t *testing.T) {
	s := newStore(t)
	mustAddKeys(t, s,
		KeyDelta{Day: "2026-11-01", Hour: 9, VK: 0x41, Count: 10},
		KeyDelta{Day: "2026-11-01", Hour: 10, VK: 0x42, Count: 5},
		KeyDelta{Day: "2026-11-02", Hour: 9, VK: 0x41, Count: 3},
	)

	got, err := s.DailyTotals("2026-11-01", "2026-11-02")
	if err != nil {
		t.Fatalf("DailyTotals: %v", err)
	}
	if got["2026-11-01"] != 15 {
		t.Errorf("11-01 = %d，期望 15", got["2026-11-01"])
	}
	if got["2026-11-02"] != 3 {
		t.Errorf("11-02 = %d，期望 3", got["2026-11-02"])
	}
}

func TestActivityAccumulates(t *testing.T) {
	s := newStore(t)

	// 同一天分两次落库（模拟两次 flush）
	if err := s.AddActivity("2026-11-02", 30, "09:00:00", "09:00:30"); err != nil {
		t.Fatalf("AddActivity: %v", err)
	}
	if err := s.AddActivity("2026-11-02", 45, "08:10:00", "09:01:15"); err != nil {
		t.Fatalf("AddActivity: %v", err)
	}

	total, err := s.ActiveSeconds("2026-11-02", "2026-11-02")
	if err != nil {
		t.Fatalf("ActiveSeconds: %v", err)
	}
	if total != 75 {
		t.Fatalf("活跃秒数 = %d，期望 75", total)
	}

	first, last, err := s.PressBounds("2026-11-02", "2026-11-02")
	if err != nil {
		t.Fatalf("PressBounds: %v", err)
	}
	if first != "08:10:00" {
		t.Errorf("首次按键 = %q，期望 08:10:00（应保留更早的）", first)
	}
	if last != "09:01:15" {
		t.Errorf("最后按键 = %q，期望 09:01:15（应保留更晚的）", last)
	}
}

func TestFirstDay(t *testing.T) {
	s := newStore(t)

	day, err := s.FirstDay()
	if err != nil {
		t.Fatalf("FirstDay: %v", err)
	}
	if day != "" {
		t.Fatalf("空库 FirstDay = %q，期望空字符串", day)
	}

	mustAddKeys(t, s,
		KeyDelta{Day: "2026-10-05", Hour: 1, VK: 0x41, Count: 1},
		KeyDelta{Day: "2026-10-01", Hour: 1, VK: 0x41, Count: 1},
	)

	day, err = s.FirstDay()
	if err != nil {
		t.Fatalf("FirstDay: %v", err)
	}
	if day != "2026-10-01" {
		t.Fatalf("FirstDay = %q，期望 2026-10-01", day)
	}
}

func TestClearAll(t *testing.T) {
	s := newStore(t)
	mustAddKeys(t, s, KeyDelta{Day: "2026-11-02", Hour: 9, VK: 0x41, Count: 10})
	if err := s.AddComboCounts([]ComboDelta{{Day: "2026-11-02", Hour: 9, Combo: "Ctrl+C", Count: 2}}); err != nil {
		t.Fatalf("AddComboCounts: %v", err)
	}
	if err := s.AddActivity("2026-11-02", 60, "09:00:00", "09:01:00"); err != nil {
		t.Fatalf("AddActivity: %v", err)
	}

	if err := s.ClearAll(); err != nil {
		t.Fatalf("ClearAll: %v", err)
	}

	keys, err := s.KeyCounts("2000-01-01", "2999-12-31")
	if err != nil {
		t.Fatalf("KeyCounts: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("清空后仍有按键数据: %v", keys)
	}

	combos, err := s.ComboCounts("2000-01-01", "2999-12-31")
	if err != nil {
		t.Fatalf("ComboCounts: %v", err)
	}
	if len(combos) != 0 {
		t.Errorf("清空后仍有组合键数据: %v", combos)
	}

	sec, err := s.ActiveSeconds("2000-01-01", "2999-12-31")
	if err != nil {
		t.Fatalf("ActiveSeconds: %v", err)
	}
	if sec != 0 {
		t.Errorf("清空后活跃秒数 = %d，期望 0", sec)
	}
}

func TestMeta(t *testing.T) {
	s := newStore(t)

	_, ok, err := s.Meta("onboarded")
	if err != nil {
		t.Fatalf("Meta: %v", err)
	}
	if ok {
		t.Fatal("新库里不该有 onboarded")
	}

	if err := s.SetMeta("collecting", "0"); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	v, ok, err := s.Meta("collecting")
	if err != nil {
		t.Fatalf("Meta: %v", err)
	}
	if !ok || v != "0" {
		t.Fatalf("Meta(collecting) = %q/%v，期望 \"0\"/true", v, ok)
	}

	// 覆盖写
	if err := s.SetMeta("collecting", "1"); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	v, _, err = s.Meta("collecting")
	if err != nil {
		t.Fatalf("Meta: %v", err)
	}
	if v != "1" {
		t.Fatalf("Meta(collecting) = %q，期望 \"1\"", v)
	}
}

func TestSchemaVersionIsWritten(t *testing.T) {
	s := newStore(t)
	v, ok, err := s.Meta("schema_version")
	if err != nil {
		t.Fatalf("Meta: %v", err)
	}
	if !ok || v != "1" {
		t.Fatalf("schema_version = %q/%v，期望 \"1\"/true", v, ok)
	}
}

func mustAddKeys(t *testing.T, s *Store, deltas ...KeyDelta) {
	t.Helper()
	if err := s.AddKeyCounts(deltas); err != nil {
		t.Fatalf("AddKeyCounts: %v", err)
	}
}
