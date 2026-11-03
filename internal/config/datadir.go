package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// portableMarker 是便携模式标记：exe 同目录出现它，数据就放在程序边上。
	// 由安装包在用户勾选「便携模式」时创建，用户也可以手工放一个。
	portableMarker = "portable.txt"
	// portableDataDirName 是便携模式的数据子目录，避免和程序文件混在一起。
	portableDataDirName = "data"
	// dataDirSetting 是注册表里保存「自定义数据目录」的值名。
	dataDirSetting = "DataDir"
)

// DataSource 说明数据目录是怎么定下来的，用于界面提示与排障。
type DataSource int

const (
	// SourceDefault 是 %LOCALAPPDATA%\keyboard-statistics。
	SourceDefault DataSource = iota
	// SourcePortable 是 <exe 目录>\data，由 portable.txt 触发。
	SourcePortable
	// SourceSetting 是用户在设置里指定的目录。
	SourceSetting
)

func (s DataSource) String() string {
	switch s {
	case SourceSetting:
		return "自定义目录"
	case SourcePortable:
		return "便携模式（程序目录）"
	default:
		return "用户数据目录"
	}
}

// SettingStore 抽象「读写一个字符串设置」，真实实现是 HKCU 注册表，测试用内存实现。
type SettingStore interface {
	Get(name string) (value string, ok bool, err error)
	Set(name, value string) error
}

// ResolveDataDir 决定数据目录，优先级从高到低：
//
//  1. 用户在设置里显式指定的目录（HKCU\Software\keyboard-statistics\DataDir）
//  2. 便携模式：exe 同目录有 portable.txt 且目录确实可写
//  3. 默认 %LOCALAPPDATA%\keyboard-statistics
//
// 显式设置即使当前不可写也照用不误：用户可能只是把盘挂载晚了，
// 这时静默换到别处会把数据悄悄写散到两个地方，不如让调用方报错。
// 读设置失败不阻断启动——退回默认目录，程序先跑起来。
func ResolveDataDir(exePath string, store SettingStore) (string, DataSource, error) {
	if store != nil {
		if v, ok, err := store.Get(dataDirSetting); err == nil && ok {
			if dir := strings.TrimSpace(v); dir != "" {
				return dir, SourceSetting, nil
			}
		}
	}
	if dir, ok := PortableDir(exePath); ok {
		return dir, SourcePortable, nil
	}
	dir, err := Dir()
	if err != nil {
		return "", SourceDefault, err
	}
	return dir, SourceDefault, nil
}

// FallbackDir 返回「没有用户指定目录时」会用到的位置。
//
// 用户改了数据目录后，旧数据还躺在原来的位置，迁移时得知道去哪儿找。
func FallbackDir(exePath string) (string, DataSource, error) {
	if dir, ok := PortableDir(exePath); ok {
		return dir, SourcePortable, nil
	}
	dir, err := Dir()
	if err != nil {
		return "", SourceDefault, err
	}
	return dir, SourceDefault, nil
}

// MigrateIfNeeded 在「新位置还没有数据、旧位置有数据」时把旧数据搬过去。
//
// 只在启动早期调用：那时数据库还没打开，文件没有句柄占用，搬动才是安全的。
// 目标已有统计库时一律不动，避免把用户在新位置产生的数据冲掉。
func MigrateIfNeeded(fallback, target string) (bool, error) {
	if samePath(fallback, target) {
		return false, nil
	}
	if _, err := os.Stat(filepath.Join(target, "stats.db")); err == nil {
		return false, nil
	}
	if _, err := os.Stat(filepath.Join(fallback, "stats.db")); err != nil {
		return false, nil // 旧位置本来就没有数据（首次安装、或已经搬过）
	}
	if err := MigrateData(fallback, target); err != nil {
		return false, err
	}
	return true, nil
}

// PortableDir 在便携模式且目录可写时返回 <exe 目录>\data。
//
// 可写性必须实测：装到 Program Files 时标记文件可能存在，但数据写不进去，
// 这时要老老实实回退，不能让程序起不来。
func PortableDir(exePath string) (string, bool) {
	appDir := filepath.Dir(exePath)
	if _, err := os.Stat(filepath.Join(appDir, portableMarker)); err != nil {
		return "", false
	}
	dir := filepath.Join(appDir, portableDataDirName)
	if err := EnsureWritable(dir); err != nil {
		return "", false
	}
	return dir, true
}

// EnsureWritable 确保 dir 存在且真的能写。
//
// 只 MkdirAll 不够：目录可能存在但当前用户没有写权限，
// 用一次「建临时文件再删掉」来实测，比事后报「数据库打不开」清楚得多。
func EnsureWritable(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建目录 %s: %w", dir, err)
	}
	probe, err := os.CreateTemp(dir, ".write-probe-*")
	if err != nil {
		return fmt.Errorf("目录不可写 %s: %w", dir, err)
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return nil
}

// dataFiles 是随数据目录一起搬的文件。
// -wal 必须跟着搬，否则主库看起来是好的、近期数据却丢了。
var dataFiles = []string{"stats.db", "stats.db-wal", "stats.db-shm", "app.log"}

// MigrateData 把数据文件从 from 搬到 to。
//
// 目标已有统计库时直接报错：宁可让用户先确认，也不能把旧数据冲掉。
// 先全部复制成功再删源文件，复制中断时至少还能从源目录找回。
func MigrateData(from, to string) error {
	if samePath(from, to) {
		return nil
	}
	if err := EnsureWritable(to); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(to, "stats.db")); err == nil {
		return fmt.Errorf("目标目录已存在统计库 %s，不覆盖", filepath.Join(to, "stats.db"))
	}

	moved := make([]string, 0, len(dataFiles))
	for _, name := range dataFiles {
		src := filepath.Join(from, name)
		if _, err := os.Stat(src); err != nil {
			continue // 文件本来就没有（比如从没写过日志），跳过
		}
		dst := filepath.Join(to, name)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("复制 %s: %w", name, err)
		}
		moved = append(moved, src)
	}
	for _, src := range moved {
		if err := os.Remove(src); err != nil {
			return fmt.Errorf("删除源文件 %s: %w", src, err)
		}
	}
	return nil
}

// samePath 判断两个目录是否指向同一处（Windows 路径大小写不敏感）。
func samePath(a, b string) bool {
	ca, err1 := filepath.Abs(a)
	cb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return strings.EqualFold(filepath.Clean(ca), filepath.Clean(cb))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// DBPathIn 返回指定数据目录下的统计库路径。
func DBPathIn(dir string) string { return filepath.Join(dir, "stats.db") }

// SetDataDir 持久化用户指定的数据目录。
//
// 真正的搬文件不在这里做：调用时数据库通常还开着，Windows 不允许移动被占用的文件。
func SetDataDir(store SettingStore, dir string) error {
	if store == nil {
		return fmt.Errorf("config: 当前环境不支持保存数据目录")
	}
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("config: 数据目录不能为空")
	}
	if err := store.Set(dataDirSetting, dir); err != nil {
		return fmt.Errorf("保存数据目录设置: %w", err)
	}
	return nil
}

// LogPathIn 返回指定数据目录下的日志路径。
func LogPathIn(dir string) string { return filepath.Join(dir, "app.log") }
