"""盯住一个窗口，记录它「未响应」的时间段和 CPU 消耗。

用法：python watch_hang.py <pid> <秒数>

每 200 毫秒问一次 Windows：这个窗口现在是不是没响应（IsHungAppWindow，
就是任务管理器里显示「未响应」的那个判定），顺便采样 CPU。只读，不动目标进程。
"""
import ctypes
import ctypes.wintypes as wt
import sys
import time

MAIN_TITLE = "键盘使用统计"

user32 = ctypes.WinDLL("user32", use_last_error=True)
psapi = ctypes.WinDLL("psapi", use_last_error=True)
kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)

user32.FindWindowW.restype = wt.HWND
user32.FindWindowW.argtypes = [wt.LPCWSTR, wt.LPCWSTR]
user32.IsHungAppWindow.argtypes = [wt.HWND]
user32.IsHungAppWindow.restype = wt.BOOL
user32.SendMessageTimeoutW.restype = wt.LPARAM
user32.SendMessageTimeoutW.argtypes = [wt.HWND, wt.UINT, wt.WPARAM, wt.LPARAM, wt.UINT, wt.UINT, ctypes.POINTER(ctypes.c_size_t)]
kernel32.OpenProcess.restype = wt.HANDLE
kernel32.GetExitCodeProcess.argtypes = [wt.HANDLE, ctypes.POINTER(wt.DWORD)]
kernel32.GetProcessTimes.argtypes = [wt.HANDLE] + [ctypes.POINTER(wt.FILETIME)] * 4

try:
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
except Exception:
    pass


def cpu_seconds(pid: int) -> float:
    h = kernel32.OpenProcess(0x0400 | 0x0010, False, pid)  # QUERY_INFORMATION | VM_READ
    if not h:
        return -1.0
    times = [wt.FILETIME() for _ in range(4)]
    ok = kernel32.GetProcessTimes(h, *[ctypes.byref(t) for t in times])
    kernel32.CloseHandle(h)
    if not ok:
        return -1.0

    def to_s(ft):
        return (ft.dwHighDateTime << 32 | ft.dwLowDateTime) / 10_000_000

    return to_s(times[0]) + to_s(times[1])


def main() -> int:
    pid = int(sys.argv[1])
    total = float(sys.argv[2]) if len(sys.argv) > 2 else 120.0

    start = time.time()
    hung_since = None
    hangs = []
    last_cpu = cpu_seconds(pid)
    last_t = time.time()
    samples = 0

    while time.time() - start < total:
        hwnd = user32.FindWindowW(None, MAIN_TITLE)
        if not hwnd:
            print(f"[{time.time()-start:6.1f}s] 窗口不在了（程序退出？）")
            break

        hung = bool(user32.IsHungAppWindow(hwnd))
        if hung and hung_since is None:
            hung_since = time.time()
            print(f"[{hung_since-start:6.1f}s] 开始未响应")
        elif not hung and hung_since is not None:
            dur = time.time() - hung_since
            hangs.append(dur)
            print(f"[{time.time()-start:6.1f}s] 恢复响应（持续 {dur:.1f}s）")
            hung_since = None

        if samples % 15 == 0:
            now = time.time()
            c = cpu_seconds(pid)
            if c >= 0 and last_cpu >= 0:
                print(f"[{now-start:6.1f}s] CPU {c-last_cpu:5.2f}s / {now-last_t:5.1f}s  未响应={hung}")
            last_cpu, last_t = c, now
        samples += 1
        time.sleep(0.2)

    if hung_since is not None:
        hangs.append(time.time() - hung_since)
        print(f"结束时仍在未响应，已持续 {hangs[-1]:.1f}s")

    print(f"\n共观测到 {len(hangs)} 段未响应" + (f"：{[round(h,1) for h in hangs]}" if hangs else ""))
    return 0 if not hangs else 1


if __name__ == "__main__":
    raise SystemExit(main())
