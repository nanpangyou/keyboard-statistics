package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirUsesLocalAppData(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmp)

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	want := filepath.Join(tmp, "keyboard-statistics")
	if dir != want {
		t.Fatalf("Dir = %q，期望 %q", dir, want)
	}
}

func TestDataPaths(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmp)

	db, err := DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	if filepath.Base(db) != "stats.db" {
		t.Errorf("DBPath = %q，期望以 stats.db 结尾", db)
	}

	logPath, err := LogPath()
	if err != nil {
		t.Fatalf("LogPath: %v", err)
	}
	if filepath.Base(logPath) != "app.log" {
		t.Errorf("LogPath = %q，期望以 app.log 结尾", logPath)
	}
}

func TestEnsureDirCreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmp)

	dir, err := EnsureDir()
	if err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("目录没建出来: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%q 不是目录", dir)
	}
}

// fakeRegistry 用内存 map 模拟 HKCU\...\Run。
type fakeRegistry struct {
	values map[string]string
	gets   int
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{values: make(map[string]string)}
}

func (f *fakeRegistry) Get(name string) (string, bool, error) {
	f.gets++
	v, ok := f.values[name]
	return v, ok, nil
}

func (f *fakeRegistry) Set(name, value string) error {
	f.values[name] = value
	return nil
}

func (f *fakeRegistry) Delete(name string) error {
	delete(f.values, name)
	return nil
}

func TestCommandLine(t *testing.T) {
	tests := []struct {
		exe  string
		want string
	}{
		{`C:\Tools\app.exe`, `"C:\Tools\app.exe" --autostart`},
		{`C:\Program Files\keyboard statistics\app.exe`, `"C:\Program Files\keyboard statistics\app.exe" --autostart`},
	}
	for _, tt := range tests {
		if got := CommandLine(tt.exe); got != tt.want {
			t.Errorf("CommandLine(%q) = %q，期望 %q", tt.exe, got, tt.want)
		}
	}
}

func TestAutostartEnableDisable(t *testing.T) {
	reg := newFakeRegistry()
	a := NewAutostart(reg, `C:\Tools\app.exe`)

	enabled, err := a.Enabled()
	if err != nil {
		t.Fatalf("Enabled: %v", err)
	}
	if enabled {
		t.Fatal("初始状态不该是已启用")
	}

	if err := a.Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if got := reg.values[runValueName]; got != `"C:\Tools\app.exe" --autostart` {
		t.Fatalf("注册表值 = %q", got)
	}

	enabled, err = a.Enabled()
	if err != nil {
		t.Fatalf("Enabled: %v", err)
	}
	if !enabled {
		t.Fatal("Enable 之后 Enabled 应为 true")
	}

	if err := a.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if _, ok := reg.values[runValueName]; ok {
		t.Fatal("Disable 之后注册表值应被删除")
	}
}

func TestAutostartStale(t *testing.T) {
	reg := newFakeRegistry()
	a := NewAutostart(reg, `C:\Old\app.exe`)

	if err := a.Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}

	// 用户把 exe 挪到了新位置
	moved := NewAutostart(reg, `D:\New\app.exe`)
	stale, err := moved.Stale()
	if err != nil {
		t.Fatalf("Stale: %v", err)
	}
	if !stale {
		t.Fatal("路径已变，应判定为失效")
	}

	same := NewAutostart(reg, `C:\Old\app.exe`)
	stale, err = same.Stale()
	if err != nil {
		t.Fatalf("Stale: %v", err)
	}
	if stale {
		t.Fatal("路径未变，不该判定为失效")
	}
}

func TestAutostartStaleWhenDisabled(t *testing.T) {
	reg := newFakeRegistry()
	a := NewAutostart(reg, `C:\Tools\app.exe`)

	stale, err := a.Stale()
	if err != nil {
		t.Fatalf("Stale: %v", err)
	}
	if stale {
		t.Fatal("没启用自启时不该报失效")
	}
}

func TestParseCommandExe(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"标准带引号", `"C:\Tools\app.exe" --autostart`, `C:\Tools\app.exe`},
		{"没有参数", `"C:\Tools\app.exe"`, `C:\Tools\app.exe`},
		{"历史遗留的无引号写法", `C:\Tools\app.exe`, `C:\Tools\app.exe`},
		{"空值", ``, ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseCommandExe(tt.value); got != tt.want {
				t.Fatalf("ParseCommandExe(%q) = %q，期望 %q", tt.value, got, tt.want)
			}
		})
	}
}
