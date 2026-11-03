"""验证「首次关窗时的选项框」两个分支，以及选择有没有被记住。

用法：python close_choice.py <exe路径> <数据目录> <tray|exit>

流程：
  1. 先删掉注册表里的 CloseAction，模拟「从没选过」；
  2. 启动程序、投递 WM_CLOSE，等选项框出现（标题「关闭窗口后要怎么做？」）；
  3. 按 tray 就点「是」、按 exit 就点「否」；
  4. 断言：进程该活着还是该退出、窗口该藏还是该在、注册表有没有记下这个选择。

参数的 tray/exit 既表示「点哪个按钮」，也用来对照期望结果。
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
WM_COMMAND = 0x0111
IDYES = 6
IDNO = 7
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
user32.EnumChildWindows.argtypes = [wt.HWND, EnumProc, wt.LPARAM]
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


def child_texts(hwnd):
    out = []

    def cb(h, _):
        t = ctypes.create_unicode_buffer(1024)
        user32.GetWindowTextW(h, t, 1024)
        if t.value:
            out.append(t.value)
        return True

    user32.EnumChildWindows(hwnd, EnumProc(cb), 0)
    return out


def read_setting(name):
    try:
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, SETTINGS_KEY) as k:
            return winreg.QueryValueEx(k, name)[0]
    except FileNotFoundError:
        return None


def main() -> int:
    exe, datadir, mode = sys.argv[1], sys.argv[2], sys.argv[3]

    # 模拟「从没选过」
    try:
        with winreg.CreateKey(winreg.HKEY_CURRENT_USER, SETTINGS_KEY) as k:
            for name in ("CloseAction", "TrayHintShown"):
                try:
                    winreg.DeleteValue(k, name)
                except FileNotFoundError:
                    pass
    except OSError as e:
        print(f"FAIL 清注册表失败: {e}")
        return 1

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

    dialog = 0
    body = ""
    for _ in range(50):
        time.sleep(0.2)
        for h, cls, title, vis in windows(pid):
            if cls == "#32770" and title == CLOSE_TITLE:
                dialog = h
                body = " ".join(child_texts(h))
                break
        if dialog:
            break

    if not dialog:
        print("FAIL 首次关窗没有弹出选项框")
        proc.kill()
        return 1
    print(f"选项框出现: {CLOSE_TITLE!r}")
    print(f"  正文: {body!r}")

    user32.SendMessageW(dialog, WM_COMMAND, IDYES if mode == "tray" else IDNO, 0)
    time.sleep(2)

    live = alive(pid)
    wins = windows(pid)
    main_visible = any(c != "#32770" and t == MAIN_TITLE and v for _, c, t, v in wins)
    saved = read_setting("CloseAction")
    hint = read_setting("TrayHintShown")

    print(f"选择后: 进程存活={live} 主窗口可见={main_visible} 记住的选择={saved!r} 气泡已弹标记={hint!r}")

    if mode == "tray":
        ok = live and not main_visible and saved == "tray" and hint == "1"
        print("期望: 收进托盘（进程活着、窗口藏起来、选择被记住、首次气泡已记）")
    else:
        ok = not live and saved == "exit"
        print("期望: 直接退出（进程结束、选择被记住）")

    if live:
        proc.kill()
    print("OK" if ok else "FAIL")
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
