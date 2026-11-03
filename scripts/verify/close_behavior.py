"""验证「关闭窗口时怎么办」的三种模式。

用法：python close_behavior.py <exe路径> <数据目录> <tray|exit|ask>

它会先改注册表里的 CloseAction（ask 模式则删掉这个值），启动程序，投递 WM_CLOSE，
然后报告：进程还在不在、窗口可见不可见、有没有弹出选项框。

注意：ask 模式弹出的选项框不会在这里被点掉（外部投递消息关不掉模态框），
脚本会在报告后杀掉进程。
"""
import ctypes
import ctypes.wintypes as wt
import subprocess
import sys
import time
import winreg

MAIN_TITLE = "键盘使用统计"
CLOSE_TITLE = "关闭窗口后要怎么做？"
WM_CLOSE = 0x0010
SETTINGS_KEY = r"Software\keyboard-statistics"

user32 = ctypes.WinDLL("user32", use_last_error=True)
kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)

user32.FindWindowW.restype = wt.HWND
user32.FindWindowW.argtypes = [wt.LPCWSTR, wt.LPCWSTR]
user32.IsWindowVisible.argtypes = [wt.HWND]
user32.PostMessageW.argtypes = [wt.HWND, wt.UINT, wt.WPARAM, wt.LPARAM]
user32.GetWindowTextW.argtypes = [wt.HWND, wt.LPWSTR, ctypes.c_int]
user32.GetClassNameW.argtypes = [wt.HWND, wt.LPWSTR, ctypes.c_int]
user32.GetWindowThreadProcessId.argtypes = [wt.HWND, ctypes.POINTER(wt.DWORD)]
kernel32.OpenProcess.restype = wt.HANDLE
EnumWindowsProc = ctypes.WINFUNCTYPE(wt.BOOL, wt.HWND, wt.LPARAM)


def alive(pid: int) -> bool:
    h = kernel32.OpenProcess(0x1000, False, pid)
    if not h:
        return False
    code = wt.DWORD()
    kernel32.GetExitCodeProcess(h, ctypes.byref(code))
    kernel32.CloseHandle(h)
    return code.value == 259


def top_windows(pid: int):
    """列出该进程的所有顶层窗口。"""
    out = []

    def cb(hwnd, _):
        p = wt.DWORD()
        user32.GetWindowThreadProcessId(hwnd, ctypes.byref(p))
        if p.value == pid:
            t = ctypes.create_unicode_buffer(256)
            user32.GetWindowTextW(hwnd, t, 256)
            c = ctypes.create_unicode_buffer(128)
            user32.GetClassNameW(hwnd, c, 128)
            out.append((hwnd, c.value, t.value, bool(user32.IsWindowVisible(hwnd))))
        return True

    user32.EnumWindows(EnumWindowsProc(cb), 0)
    return out


def set_close_action(mode: str) -> None:
    with winreg.CreateKey(winreg.HKEY_CURRENT_USER, SETTINGS_KEY) as k:
        if mode == "ask":
            try:
                winreg.DeleteValue(k, "CloseAction")
                print("注册表: 已删除 CloseAction（模拟没选过）")
            except FileNotFoundError:
                print("注册表: CloseAction 本来就不存在")
        else:
            winreg.SetValueEx(k, "CloseAction", 0, winreg.REG_SZ, mode)
            print(f"注册表: CloseAction = {mode}")


def main() -> int:
    exe, datadir, mode = sys.argv[1], sys.argv[2], sys.argv[3]
    set_close_action(mode)

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

    time.sleep(0.5)
    user32.PostMessageW(hwnd, WM_CLOSE, 0, 0)
    time.sleep(1.5)

    live = alive(pid)
    wins = top_windows(pid)
    main_visible = any(t == MAIN_TITLE and v for _, _, t, v in wins)
    dialog = [t for _, c, t, _ in wins if c == "#32770" or t == CLOSE_TITLE]

    print(f"进程存活={live} 主窗口可见={main_visible}")
    print(f"顶层窗口: {[(c, t, v) for _, c, t, v in wins]}")

    ok = False
    if mode == "exit":
        ok = not live and not wins
        print("期望: 直接退出（无窗口、无弹框）")
    elif mode == "tray":
        ok = live and not main_visible
        print("期望: 收进托盘（进程活着、主窗口不可见）")
    else:
        ok = live and CLOSE_TITLE in dialog
        print("期望: 弹出选项框，程序不动")

    print("OK" if ok else "FAIL")
    if live:
        proc.kill()
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
