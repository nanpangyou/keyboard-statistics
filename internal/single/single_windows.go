//go:build windows

package single

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutex = kernel32.NewProc("CreateMutexW")
	procCloseHandle = kernel32.NewProc("CloseHandle")
)

// errorAlreadyExists 是 CreateMutexW 在同名互斥体已存在时留下的 LastError。
// 它不是失败：句柄有效，只是告诉我们「已经有别人在用这个名字」。
const errorAlreadyExists syscall.Errno = 183

// Guard 持有单实例互斥体。必须调用 Release，否则句柄泄漏。
type Guard struct {
	handle syscall.Handle
	closed bool
}

// Acquire 尝试取得单实例锁。
//
// 返回 already=true 表示已有实例在运行，此时 guard 为 nil；
// 返回 error 表示检查本身出了问题（调用方一般应放行并记日志，
// 而不是因为拿不到锁就把程序打死）。
func Acquire(name string) (*Guard, bool, error) {
	if name == "" {
		return nil, false, fmt.Errorf("single: 互斥体名不能为空")
	}
	ptr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, false, fmt.Errorf("single: 互斥体名 %q 编码失败: %w", name, err)
	}

	handle, _, callErr := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(ptr)))
	if handle == 0 {
		return nil, false, fmt.Errorf("single: 创建互斥体 %q 失败: %w", name, callErr)
	}
	if errno, ok := callErr.(syscall.Errno); ok && errno == errorAlreadyExists {
		// 抢锁失败时句柄仍然有效（指向同一个内核对象），必须关掉：
		// 留着它等于给这个对象多记一个引用，前一个实例退出后锁也不会消失。
		procCloseHandle.Call(handle)
		return nil, true, nil
	}
	return &Guard{handle: syscall.Handle(handle)}, false, nil
}

// Release 释放互斥体。可以重复调用。
func (g *Guard) Release() {
	if g == nil || g.closed {
		return
	}
	g.closed = true
	procCloseHandle.Call(uintptr(g.handle))
}
