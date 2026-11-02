package service

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"keyboard-statistics/internal/stats"
)

// testClock 让测试可以拨动时间。
type testClock struct{ t time.Time }

func (c *testClock) now() time.Time          { return c.t }
func (c *testClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestService(t *testing.T) (*Service, *testClock) {
	t.Helper()
	clock := &testClock{t: time.Date(2026, 11, 2, 9, 0, 0, 0, time.Local)}
	svc, err := New(Options{
		DBPath:    filepath.Join(t.TempDir(), "test.db"),
		Now:       clock.now,
		StartHook: false, // 单元测试不装真钩子
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		if err := svc.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
	return svc, clock
}

func TestSnapshotReflectsPresses(t *testing.T) {
	svc, _ := newTestService(t)

	svc.KeyDown(0x41) // A
	svc.KeyUp(0x41)
	svc.KeyDown(0x42) // B
	svc.KeyUp(0x42)
	svc.KeyDown(0x42)
	svc.KeyUp(0x42)

	if err := svc.FlushNow(); err != nil {
		t.Fatalf("FlushNow: %v", err)
	}

	snap, err := svc.Snapshot(stats.ScopeToday)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if snap.TotalKeys != 3 {
		t.Errorf("总按键 = %d，期望 3", snap.TotalKeys)
	}
	if len(snap.TopKeys) != 2 {
		t.Fatalf("TOP 键数量 = %d，期望 2", len(snap.TopKeys))
	}
	if snap.TopKeys[0].VK != 0x42 || snap.TopKeys[0].Count != 2 {
		t.Errorf("第一名 = %+v，期望 B 键 2 次", snap.TopKeys[0])
	}
}

func TestSnapshotCombos(t *testing.T) {
	svc, _ := newTestService(t)

	svc.KeyDown(0x11) // Ctrl
	svc.KeyDown(0x56) // V
	svc.KeyUp(0x56)
	svc.KeyUp(0x11)

	if err := svc.FlushNow(); err != nil {
		t.Fatalf("FlushNow: %v", err)
	}

	snap, err := svc.Snapshot(stats.ScopeToday)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap.TopCombos) != 1 || snap.TopCombos[0].Combo != "Ctrl+V" {
		t.Fatalf("TOP 组合 = %+v，期望只有 Ctrl+V", snap.TopCombos)
	}
}

func TestSnapshotDailyAverageAndPeak(t *testing.T) {
	svc, clock := newTestService(t)

	// 今天按 10 次
	for i := 0; i < 10; i++ {
		svc.KeyDown(0x41)
		svc.KeyUp(0x41)
	}
	if err := svc.FlushNow(); err != nil {
		t.Fatalf("FlushNow: %v", err)
	}

	// 挪到三天后，再按 20 次
	clock.advance(72 * time.Hour)
	for i := 0; i < 20; i++ {
		svc.KeyDown(0x41)
		svc.KeyUp(0x41)
	}
	if err := svc.FlushNow(); err != nil {
		t.Fatalf("FlushNow: %v", err)
	}

	snap, err := svc.Snapshot(stats.Scope7Days)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if snap.Days != 7 {
		t.Errorf("天数 = %d，期望 7", snap.Days)
	}
	if snap.TotalKeys != 30 {
		t.Errorf("总按键 = %d，期望 30", snap.TotalKeys)
	}
	// 30 / 7 = 4（向下取整）
	if snap.DailyAverage != 4 {
		t.Errorf("日均 = %d，期望 4", snap.DailyAverage)
	}
	if snap.PeakCount != 20 {
		t.Errorf("峰值 = %d，期望 20", snap.PeakCount)
	}
}

func TestPauseStopsCollecting(t *testing.T) {
	svc, _ := newTestService(t)

	svc.SetCollecting(false)
	svc.KeyDown(0x41)
	svc.KeyUp(0x41)

	if err := svc.FlushNow(); err != nil {
		t.Fatalf("FlushNow: %v", err)
	}

	snap, err := svc.Snapshot(stats.ScopeToday)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.TotalKeys != 0 {
		t.Fatalf("暂停后仍记录到 %d 次按键", snap.TotalKeys)
	}

	svc.SetCollecting(true)
	svc.KeyDown(0x41)
	svc.KeyUp(0x41)
	if err := svc.FlushNow(); err != nil {
		t.Fatalf("FlushNow: %v", err)
	}
	snap, err = svc.Snapshot(stats.ScopeToday)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.TotalKeys != 1 {
		t.Fatalf("恢复后总按键 = %d，期望 1", snap.TotalKeys)
	}
}

func TestClearAllData(t *testing.T) {
	svc, _ := newTestService(t)

	svc.KeyDown(0x41)
	svc.KeyUp(0x41)
	if err := svc.FlushNow(); err != nil {
		t.Fatalf("FlushNow: %v", err)
	}

	if err := svc.ClearData(); err != nil {
		t.Fatalf("ClearData: %v", err)
	}

	snap, err := svc.Snapshot(stats.ScopeAll)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.TotalKeys != 0 {
		t.Fatalf("清空后总按键 = %d，期望 0", snap.TotalKeys)
	}
}

func TestExportWritesTwoCSV(t *testing.T) {
	svc, _ := newTestService(t)

	svc.KeyDown(0x11)
	svc.KeyDown(0x56)
	svc.KeyUp(0x56)
	svc.KeyUp(0x11)
	if err := svc.FlushNow(); err != nil {
		t.Fatalf("FlushNow: %v", err)
	}

	dir := t.TempDir()
	files, err := svc.ExportCSV(dir)
	if err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("导出文件数 = %d，期望 2: %v", len(files), files)
	}

	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读取导出文件: %v", err)
		}
		if !strings.HasPrefix(string(raw), "\ufeff") {
			t.Errorf("%s 缺少 UTF-8 BOM，Excel 打开会乱码", filepath.Base(f))
		}
		if !strings.Contains(string(raw), "次数") {
			t.Errorf("%s 缺少表头", filepath.Base(f))
		}
	}

	keysFile := files[0]
	if !strings.Contains(filepath.Base(keysFile), "_keys.csv") {
		keysFile, _ = findFile(files, "_keys.csv")
	}
	raw, err := os.ReadFile(keysFile)
	if err != nil {
		t.Fatalf("读取按键 CSV: %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(raw), "\ufeff"))).ReadAll()
	if err != nil {
		t.Fatalf("解析按键 CSV: %v", err)
	}
	if len(records) != 3 { // 表头 + V + Ctrl
		t.Fatalf("按键 CSV 行数 = %d，期望 3：%v", len(records), records)
	}
}

func findFile(files []string, suffix string) (string, bool) {
	for _, f := range files {
		if strings.HasSuffix(f, suffix) {
			return f, true
		}
	}
	return "", false
}
