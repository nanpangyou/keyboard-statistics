package config

import (
	"os"
	"path/filepath"
	"testing"
)

// memStore 是内存版设置存储，避免测试碰真实注册表。
type memStore struct {
	values map[string]string
	err    error
}

func newMemStore() *memStore { return &memStore{values: map[string]string{}} }

func (m *memStore) Get(name string) (string, bool, error) {
	if m.err != nil {
		return "", false, m.err
	}
	v, ok := m.values[name]
	return v, ok, nil
}

func (m *memStore) Set(name, value string) error {
	if m.err != nil {
		return m.err
	}
	m.values[name] = value
	return nil
}

// exeIn 造一个「exe 放在 dir 下」的路径，并确保 dir 存在。
func exeIn(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("准备目录失败: %v", err)
	}
	return filepath.Join(dir, "keyboard-statistics.exe")
}

func TestResolveDataDirSettingWins(t *testing.T) {
	appDir := t.TempDir()
	exe := exeIn(t, appDir)
	// 便携标记存在，但用户显式指定了目录 —— 显式设置优先
	if err := os.WriteFile(filepath.Join(appDir, portableMarker), []byte("便携模式"), 0o644); err != nil {
		t.Fatal(err)
	}

	custom := t.TempDir()
	store := newMemStore()
	store.values[dataDirSetting] = custom

	dir, src, err := ResolveDataDir(exe, store)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if dir != custom {
		t.Errorf("数据目录 = %q，期望 %q（显式设置应优先于便携模式）", dir, custom)
	}
	if src != SourceSetting {
		t.Errorf("来源 = %v，期望 %v", src, SourceSetting)
	}
}

func TestResolveDataDirPortableWhenMarkerPresent(t *testing.T) {
	appDir := t.TempDir()
	exe := exeIn(t, appDir)
	if err := os.WriteFile(filepath.Join(appDir, portableMarker), []byte("便携模式"), 0o644); err != nil {
		t.Fatal(err)
	}

	dir, src, err := ResolveDataDir(exe, newMemStore())
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	want := filepath.Join(appDir, portableDataDirName)
	if dir != want {
		t.Errorf("数据目录 = %q，期望 %q", dir, want)
	}
	if src != SourcePortable {
		t.Errorf("来源 = %v，期望 %v", src, SourcePortable)
	}
}

func TestResolveDataDirDefaultWithoutMarker(t *testing.T) {
	appDir := t.TempDir()
	exe := exeIn(t, appDir)

	dir, src, err := ResolveDataDir(exe, newMemStore())
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		t.Skip("本机没有 LOCALAPPDATA，跳过默认目录断言")
	}
	want := filepath.Join(base, appDirName)
	if dir != want {
		t.Errorf("数据目录 = %q，期望 %q", dir, want)
	}
	if src != SourceDefault {
		t.Errorf("来源 = %v，期望 %v", src, SourceDefault)
	}
}

// 便携模式标记存在但程序目录写不进去（比如装在 Program Files），
// 必须回退到用户目录，而不是让程序起不来。
func TestResolveDataDirPortableNotWritableFallsBack(t *testing.T) {
	base := t.TempDir()
	// 用一个「文件」当作 exe 所在目录，MkdirAll 必然失败，以此模拟不可写
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(blocker, "keyboard-statistics.exe")

	dir, src, err := ResolveDataDir(exe, newMemStore())
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if src != SourceDefault {
		t.Errorf("来源 = %v，期望回退到 %v", src, SourceDefault)
	}
	if dir == "" {
		t.Error("回退后不该返回空目录")
	}
}

// 空字符串不算有效设置，否则注册表里一个空值就能把数据目录搞成当前目录。
func TestResolveDataDirEmptySettingIgnored(t *testing.T) {
	appDir := t.TempDir()
	exe := exeIn(t, appDir)
	store := newMemStore()
	store.values[dataDirSetting] = "   "

	_, src, err := ResolveDataDir(exe, store)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if src == SourceSetting {
		t.Error("空白设置不该被当成有效目录")
	}
}

// 读设置失败不能阻断启动：退回默认目录，让程序先跑起来。
func TestResolveDataDirReadErrorFallsBack(t *testing.T) {
	appDir := t.TempDir()
	exe := exeIn(t, appDir)
	store := newMemStore()
	store.err = os.ErrPermission

	dir, src, err := ResolveDataDir(exe, store)
	if err != nil {
		t.Fatalf("读设置失败时应回退而不是报错，实际: %v", err)
	}
	if src != SourceDefault || dir == "" {
		t.Errorf("来源 = %v 目录 = %q，期望回退到默认目录", src, dir)
	}
}

func TestEnsureWritable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")
	if err := EnsureWritable(dir); err != nil {
		t.Fatalf("可写目录不该报错: %v", err)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Errorf("EnsureWritable 应该把目录建出来，stat=%v err=%v", fi, err)
	}

	// 用文件当目录 → 必然不可写
	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureWritable(filepath.Join(blocker, "data")); err == nil {
		t.Error("不可写路径应该返回错误")
	}
}

func TestMigrateDataMovesFiles(t *testing.T) {
	from := t.TempDir()
	to := t.TempDir()

	for _, name := range []string{"stats.db", "stats.db-wal", "app.log"} {
		if err := os.WriteFile(filepath.Join(from, name), []byte("内容-"+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := MigrateData(from, to); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	for _, name := range []string{"stats.db", "stats.db-wal", "app.log"} {
		got, err := os.ReadFile(filepath.Join(to, name))
		if err != nil {
			t.Fatalf("目标目录缺少 %s: %v", name, err)
		}
		if string(got) != "内容-"+name {
			t.Errorf("%s 内容不对: %q", name, got)
		}
		if _, err := os.Stat(filepath.Join(from, name)); !os.IsNotExist(err) {
			t.Errorf("迁移后源文件 %s 应该已删除", name)
		}
	}
}

// 目标目录已经有库时不能覆盖，否则用户的旧数据会被悄悄冲掉。
func TestMigrateDataRefusesExistingDatabase(t *testing.T) {
	from := t.TempDir()
	to := t.TempDir()

	if err := os.WriteFile(filepath.Join(from, "stats.db"), []byte("旧"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(to, "stats.db"), []byte("已存在"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MigrateData(from, to); err == nil {
		t.Error("目标已有数据库时必须报错，不能覆盖")
	}
	got, err := os.ReadFile(filepath.Join(to, "stats.db"))
	if err != nil || string(got) != "已存在" {
		t.Errorf("目标库被改动了: %q err=%v", got, err)
	}
}

func TestMigrateDataSameDirIsNoop(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stats.db"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MigrateData(dir, dir); err != nil {
		t.Fatalf("同目录迁移应该直接返回，实际: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "stats.db")); err != nil {
		t.Errorf("同目录迁移不该动文件: %v", err)
	}
}

// FallbackDir 是「没有用户指定时」的位置，迁移时要知道旧数据在哪。
func TestFallbackDirIgnoresSetting(t *testing.T) {
	appDir := t.TempDir()
	exe := exeIn(t, appDir)
	if err := os.WriteFile(filepath.Join(appDir, portableMarker), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	custom := t.TempDir()
	store := newMemStore()
	store.values[dataDirSetting] = custom

	dir, src, err := FallbackDir(exe)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if dir == custom {
		t.Error("FallbackDir 不该返回用户指定的目录")
	}
	if src != SourcePortable {
		t.Errorf("来源 = %v，期望 %v", src, SourcePortable)
	}
}

func TestMigrateIfNeededMovesOldData(t *testing.T) {
	fallback := t.TempDir()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(fallback, "stats.db"), []byte("旧数据"), 0o644); err != nil {
		t.Fatal(err)
	}

	moved, err := MigrateIfNeeded(fallback, target)
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	if !moved {
		t.Fatal("旧位置有库、新位置没有，应该迁移")
	}
	got, err := os.ReadFile(filepath.Join(target, "stats.db"))
	if err != nil || string(got) != "旧数据" {
		t.Errorf("目标库内容不对: %q err=%v", got, err)
	}
}

func TestMigrateIfNeededSkipsWhenTargetHasData(t *testing.T) {
	fallback := t.TempDir()
	target := t.TempDir()
	for dir, content := range map[string]string{fallback: "旧", target: "新"} {
		if err := os.WriteFile(filepath.Join(dir, "stats.db"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	moved, err := MigrateIfNeeded(fallback, target)
	if err != nil {
		t.Fatalf("不该报错: %v", err)
	}
	if moved {
		t.Error("目标已有库时不该搬，否则会覆盖用户数据")
	}
	got, _ := os.ReadFile(filepath.Join(target, "stats.db"))
	if string(got) != "新" {
		t.Errorf("目标库被改动了: %q", got)
	}
}

func TestMigrateIfNeededNoOldData(t *testing.T) {
	fallback := t.TempDir()
	target := t.TempDir()

	moved, err := MigrateIfNeeded(fallback, target)
	if err != nil {
		t.Fatalf("不该报错: %v", err)
	}
	if moved {
		t.Error("旧位置没有库时不该报告迁移过")
	}
}

func TestMigrateIfNeededSameDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stats.db"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	moved, err := MigrateIfNeeded(dir, dir)
	if err != nil || moved {
		t.Errorf("同目录应直接跳过: moved=%v err=%v", moved, err)
	}
}

func TestSetDataDirRoundTrip(t *testing.T) {
	store := newMemStore()
	want := filepath.Join(t.TempDir(), "数据")

	if err := SetDataDir(store, want); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	appDir := t.TempDir()
	exe := exeIn(t, appDir)
	dir, src, err := ResolveDataDir(exe, store)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if dir != want || src != SourceSetting {
		t.Errorf("保存后再解析得到 (%q, %v)，期望 (%q, %v)", dir, src, want, SourceSetting)
	}
}

func TestSetDataDirRejectsEmpty(t *testing.T) {
	if err := SetDataDir(newMemStore(), "   "); err == nil {
		t.Error("空目录应该报错")
	}
	if err := SetDataDir(nil, `D:\x`); err == nil {
		t.Error("没有设置存储时应该报错，而不是静默丢弃")
	}
}
