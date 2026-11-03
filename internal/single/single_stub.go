//go:build !windows

package single

// Guard 在非 Windows 平台没有实际作用，只为让上层代码在别的平台也能编译。
type Guard struct{}

// Acquire 在非 Windows 平台总是成功：单实例只对 Windows 桌面程序有意义，
// 而这里存在主要是为了让 `go test ./...` 能在任何平台跑。
func Acquire(name string) (*Guard, bool, error) {
	return &Guard{}, false, nil
}

// Release 什么也不做。
func (g *Guard) Release() {}
