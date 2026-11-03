"""复现 / 验证「托盘右键菜单把界面线程卡死」。

用法：python tray_menu_hang.py <exe路径> <数据目录> [hidden|visible]

做的是用户那几步：启动 → （可选）收进托盘 → 给主窗口投递一条托盘右键消息，
然后盯 12 秒看窗口有没有变成「未响应」。

托盘回调消息是 0x8002（wmApp+2），lParam 直接就是鼠标消息 WM_RBUTTONUP(0x0205)，
因为我们没有调 NIM_SETVERSION。
"""
import ctypes
import ctypes.wintypes as wt
import subprocess
import sys
import time

MAIN_TITLE = "键盘使用统计"
MSG_TRAY = 0x8002
WM_LBUTTONUP = 0x0202
WM_RBUTTONUP = 0x0205
WM_CLOSE = 0x0010

user32 = ctypes.WinDLL("user32", use_last_error=True)
kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)

user32.FindWindowW.restype = wt.HWND
user32.FindWindowW.argtypes = [wt.LPCWSTR, wt.LPCWSTR]
user32.PostMessageW.argtypes = [wt.HWND, wt.UINT, wt.WPARAM, wt.LPARAM]
user32.IsHungAppWindow.argtypes = [wt.HWND]
user32.IsWindowVisible.argtypes = [wt.HWND]
kernel32.OpenProcess.restype = wt.HANDLE

try:
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
except Exception:
    pass


def alive(pid: int) -> bool:
    h = kernel32.OpenProcess(0x1000, False, pid)
    if not h:
        return False
    code = wt.DWORD()
    kernel32.GetExitCodeProcess(h, ctypes.byref(code))
    kernel32.CloseHandle(h)
    return code.value == 259


def main() -> int:
    exe, datadir = sys.argv[1], sys.argv[2]
    mode = sys.argv[3] if len(sys.argv) > 3 else "hidden"

    proc = subprocess.Popen([exe], cwd=datadir)
    pid = proc.pid

    hwnd = 0
    for _ in range(60):
        time.sleep(0.2)
        hwnd = user32.FindWindowW(None, MAIN_TITLE)
        if hwnd:
            break
    if not hwnd:
        print("FAIL 主窗口没出现")
        proc.kill()
        return 1
    print(f"窗口出现 hwnd={hwnd} pid={pid}")

    if mode == "hidden":
        user32.PostMessageW(hwnd, WM_CLOSE, 0, 0)
        # 等到真的隐藏（关窗行为可能是弹框，那也算没隐藏，直接报告）
        for _ in range(30):
            time.sleep(0.2)
            if not user32.IsWindowVisible(hwnd):
                break
        print(f"收进托盘: 可见={bool(user32.IsWindowVisible(hwnd))} 进程存活={alive(pid)}")

    print("投递托盘右键消息 ...")
    user32.PostMessageW(hwnd, MSG_TRAY, 0, WM_RBUTTONUP)

    hung_at = 0.0
    t0 = time.time()
    while time.time() - t0 < 12:
        time.sleep(0.3)
        if not alive(pid):
            print("进程退出了")
            break
        if user32.IsHungAppWindow(hwnd):
            hung_at = time.time() - t0
            break

    if hung_at:
        print(f"FAIL 右键之后 {(hung_at):.1f} 秒窗口变成未响应（界面线程被卡住）")
        # 再确认它是不是永久卡住：再等 5 秒看会不会自己恢复
        time.sleep(5)
        still = bool(user32.IsHungAppWindow(hwnd)) and alive(pid)
        print(f"     再等 5 秒后仍卡住={still}")
        proc.kill()
        return 1

    print("OK 右键之后界面线程仍然响应")
    proc.kill()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
