"""验证开机自启的行为：静默待在托盘，不弹窗也不弹气泡；欢迎说明推迟到用户主动打开。

用法：python autostart_silent.py <exe路径> <数据目录>

断言：
  1. `-autostart` 启动后没有可见窗口、没有任何对话框；
  2. 此时 TrayHintShown 没被写（自启不弹气泡）；
  3. 模拟点一下托盘图标 → 窗口出现，并且这时才补上欢迎说明。
"""
import ctypes
import ctypes.wintypes as wt
import subprocess
import sys
import time
import winreg

MAIN_TITLE = "键盘使用统计"
MSG_TRAY = 0x8002
WM_LBUTTONUP = 0x0202
WM_CLOSE = 0x0010
SETTINGS_KEY = r"Software\keyboard-statistics"

user32 = ctypes.WinDLL("user32", use_last_error=True)
kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)
EnumProc = ctypes.WINFUNCTYPE(wt.BOOL, wt.HWND, wt.LPARAM)

user32.FindWindowW.restype = wt.HWND
user32.FindWindowW.argtypes = [wt.LPCWSTR, wt.LPCWSTR]
user32.PostMessageW.argtypes = [wt.HWND, wt.UINT, wt.WPARAM, wt.LPARAM]
user32.SendMessageW.argtypes = [wt.HWND, wt.UINT, wt.WPARAM, wt.LPARAM]
user32.GetWindowTextW.argtypes = [wt.HWND, wt.LPWSTR, ctypes.c_int]
user32.GetClassNameW.argtypes = [wt.HWND, wt.LPWSTR, ctypes.c_int]
user32.GetWindowThreadProcessId.argtypes = [wt.HWND, ctypes.POINTER(wt.DWORD)]
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


def windows(pid: int):
    out = []

    def cb(hwnd, _):
        p = wt.DWORD()
        user32.GetWindowThreadProcessId(hwnd, ctypes.byref(p))
        if p.value == pid:
            c = ctypes.create_unicode_buffer(96)
            user32.GetClassNameW(hwnd, c, 96)
            t = ctypes.create_unicode_buffer(512)
            user32.GetWindowTextW(hwnd, t, 512)
            out.append((hwnd, c.value, t.value, bool(user32.IsWindowVisible(hwnd))))
        return True

    user32.EnumWindows(EnumProc(cb), 0)
    return out


def read_setting(name):
    try:
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, SETTINGS_KEY) as k:
            return winreg.QueryValueEx(k, name)[0]
    except FileNotFoundError:
        return None


def main() -> int:
    exe, datadir = sys.argv[1], sys.argv[2]

    try:
        with winreg.CreateKey(winreg.HKEY_CURRENT_USER, SETTINGS_KEY) as k:
            try:
                winreg.DeleteValue(k, "TrayHintShown")
            except FileNotFoundError:
                pass
    except OSError as e:
        print(f"FAIL 清注册表失败: {e}")
        return 1

    proc = subprocess.Popen([exe, "-autostart"], cwd=datadir)
    pid = proc.pid
    time.sleep(6)
    if not alive(pid):
        print(f"FAIL 自启后进程就没了（退出码 {proc.poll()}）")
        return 1

    wins = windows(pid)
    visible_main = [w for w in wins if w[2] == MAIN_TITLE and w[3]]
    dialogs = [w for w in wins if w[1] == "#32770" and w[3]]
    hint = read_setting("TrayHintShown")
    print(f"自启后: 进程存活=True 可见主窗口={len(visible_main)} 可见对话框={len(dialogs)} 气泡标记={hint!r}")
    ok = not visible_main and not dialogs and hint is None
    if not ok:
        print("FAIL 自启不该有任何可见窗口/对话框，也不该弹气泡")
        proc.kill()
        return 1

    # 模拟点托盘图标：应该把窗口放出来，并补上欠着的欢迎说明
    hwnd = user32.FindWindowW(None, MAIN_TITLE)
    if not hwnd:
        print("FAIL 找不到主窗口句柄（隐藏的窗口也该存在）")
        proc.kill()
        return 1
    user32.PostMessageW(hwnd, MSG_TRAY, 0, WM_LBUTTONUP)
    time.sleep(2)

    wins = windows(pid)
    shown = any(w[2] == MAIN_TITLE and w[3] for w in wins)
    dialogs = [w[2] for w in wins if w[1] == "#32770" and w[3]]
    print(f"点托盘图标后: 窗口可见={shown} 对话框={dialogs}")
    # 欢迎说明的标题是「欢迎使用键盘使用统计」，不是主窗口名，别拿主窗口名去比
    ok = shown and len(dialogs) == 1
    if not ok:
        print("FAIL 期望：窗口出现，并且这时才补上一个欢迎说明框")

    for h, cls, _, vis in wins:
        if cls == "#32770" and vis:
            user32.SendMessageW(h, WM_CLOSE, 0, 0)
    time.sleep(1)
    if alive(pid):
        proc.kill()
    print("OK" if ok else "FAIL")
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
