//go:build windows

package single

import (
	"fmt"
	"os"
	"testing"
)

// testName 给每个用例一个互斥体名。带上 pid，避免和真的在跑的程序、
// 或者同一台机器上的并行测试互相抢锁。
func testName(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("KeyboardStatistics_Test_%s_%d", t.Name(), os.Getpid())
}

func TestAcquireFirstInstanceSucceeds(t *testing.T) {
	name := testName(t)

	guard, already, err := Acquire(name)
	if err != nil {
		t.Fatalf("首次获取应成功，却报错: %v", err)
	}
	if already {
		t.Fatal("首次获取不应报告「已有实例」")
	}
	if guard == nil {
		t.Fatal("首次获取应返回非空 guard")
	}
	t.Cleanup(guard.Release)
}

// 这条是需求的核心：同一时刻只能有一个实例。
// 同一进程内第二次 CreateMutexW 会拿到 ERROR_ALREADY_EXISTS，
// 语义与「另一个进程已经在跑」一致，所以能在这里测。
func TestSecondAcquireIsRefused(t *testing.T) {
	name := testName(t)

	first, already, err := Acquire(name)
	if err != nil || already || first == nil {
		t.Fatalf("第一次获取应成功: guard=%v already=%v err=%v", first, already, err)
	}
	t.Cleanup(first.Release)

	second, already, err := Acquire(name)
	if err != nil {
		t.Fatalf("第二次获取不该报错（应正常告知已有实例）: %v", err)
	}
	if !already {
		t.Fatal("第二次获取必须报告「已有实例在运行」")
	}
	if second != nil {
		t.Error("已有实例时不应返回 guard，否则调用方可能误以为自己持锁")
	}
}

func TestReleaseAllowsReacquire(t *testing.T) {
	name := testName(t)

	first, _, err := Acquire(name)
	if err != nil || first == nil {
		t.Fatalf("首次获取失败: %v", err)
	}
	first.Release()

	second, already, err := Acquire(name)
	if err != nil {
		t.Fatalf("释放后再获取不该报错: %v", err)
	}
	if already {
		t.Fatal("前一个实例已释放，不该再报「已有实例」")
	}
	if second == nil {
		t.Fatal("释放后再获取应拿到 guard")
	}
	t.Cleanup(second.Release)
}

func TestReleaseIsIdempotent(t *testing.T) {
	name := testName(t)

	guard, _, err := Acquire(name)
	if err != nil || guard == nil {
		t.Fatalf("获取失败: %v", err)
	}
	guard.Release()
	guard.Release() // 重复释放不能 panic、不能把句柄重复关掉
}

func TestAcquireEmptyNameIsRejected(t *testing.T) {
	guard, already, err := Acquire("")
	if err == nil {
		t.Fatal("空名字必须报错，否则等于没有互斥")
	}
	if guard != nil || already {
		t.Errorf("报错时不该返回 guard/already: guard=%v already=%v", guard, already)
	}
}

// 互斥体名必须和 scripts\installer.iss 里的 AppMutex 完全一致，
// 否则安装器认不出正在运行的程序、也就不会先关掉它再升级。
// 改这里必须同步改 .iss，这条测试就是用来在改的时候绊你一跤。
func TestDefaultNameMatchesInstallerAppMutex(t *testing.T) {
	const want = "KeyboardStatistics_SingleInstance"
	if DefaultName != want {
		t.Fatalf("DefaultName = %q，期望 %q；\n"+
			"改动它必须同步改 scripts\\installer.iss 的 AppMutex，否则安装器无法在升级前关闭正在运行的程序",
			DefaultName, want)
	}
}
