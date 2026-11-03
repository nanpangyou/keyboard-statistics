"""验证「升级/重装时程序正在运行，先问一句」这条交互路径。

用法：python installer_prompt.py <安装包.exe> <已安装的exe路径> <yes|no>

前置条件：程序已经装好并且正在运行（不管窗口是露着还是收在托盘里）。

  1. 不带 /SILENT 启动安装包（交互模式才会弹框）；
  2. 找到那个确认框，把正文打出来，确认它说了「正在运行」和「退出」；
  3. 按 yes 就点「是」、按 no 就点「否」；
  4. 报告：确认框有没有出现、选择后的结果。

找框不能按安装包的 pid 找：Inno 会把真正干活的代码解到 %TEMP% 里另起一个
`*-setup.tmp` 子进程，弹框的是那个子进程。所以这里按「标题是 Setup 且正文提到
程序正在运行」来认人。
"""
import ctypes
import ctypes.wintypes as wt
import subprocess
import sys
import time

IDYES = 6
IDNO = 7
WM_COMMAND = 0x0111
WM_CLOSE = 0x0010
BM_CLICK = 0x00F5

user32 = ctypes.WinDLL("user32", use_last_error=True)
kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)

EnumProc = ctypes.WINFUNCTYPE(wt.BOOL, wt.HWND, wt.LPARAM)

user32.SendMessageW.argtypes = [wt.HWND, wt.UINT, wt.WPARAM, wt.LPARAM]
user32.GetWindowTextW.argtypes = [wt.HWND, wt.LPWSTR, ctypes.c_int]
user32.GetClassNameW.argtypes = [wt.HWND, wt.LPWSTR, ctypes.c_int]
user32.GetWindowThreadProcessId.argtypes = [wt.HWND, ctypes.POINTER(wt.DWORD)]
user32.IsWindowVisible.argtypes = [wt.HWND]
user32.IsWindowEnabled.argtypes = [wt.HWND]
user32.EnumChildWindows.argtypes = [wt.HWND, EnumProc, wt.LPARAM]
kernel32.OpenProcess.restype = wt.HANDLE

try:  # 控制台可能是 GBK，正文里的符号会把它噎住
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


def visible_dialogs():
    """所有可见的 #32770 对话框 (hwnd, title)。"""
    out = []

    def cb(hwnd, _):
        c = ctypes.create_unicode_buffer(96)
        user32.GetClassNameW(hwnd, c, 96)
        if c.value == "#32770" and user32.IsWindowVisible(hwnd):
            t = ctypes.create_unicode_buffer(512)
            user32.GetWindowTextW(hwnd, t, 512)
            out.append((hwnd, t.value))
        return True

    user32.EnumWindows(EnumProc(cb), 0)
    return out


def child_texts(hwnd) -> list:
    out = []

    def cb(h, _):
        t = ctypes.create_unicode_buffer(1024)
        user32.GetWindowTextW(h, t, 1024)
        if t.value:
            out.append(t.value)
        return True

    user32.EnumChildWindows(hwnd, EnumProc(cb), 0)
    return out


def find_prompt(timeout=30.0):
    """等确认框出现：正文里必须提到程序正在运行。"""
    deadline = time.time() + timeout
    while time.time() < deadline:
        for hwnd, title in visible_dialogs():
            body = " ".join(child_texts(hwnd))
            if "正在运行" in body and "退出" in body:
                return hwnd, title, body
        time.sleep(0.2)
    return 0, "", ""


def find_wizard():
    """找 Inno 的向导窗口（类名 TWizardForm，不是 #32770，别跟对话框混了）。"""
    found = []

    def cb(hwnd, _):
        c = ctypes.create_unicode_buffer(96)
        user32.GetClassNameW(hwnd, c, 96)
        if c.value == "TWizardForm" and user32.IsWindowVisible(hwnd):
            found.append(hwnd)
        return True

    user32.EnumWindows(EnumProc(cb), 0)
    return found[0] if found else 0


def click_forward(wiz) -> bool:
    """点向导上「下一步 / 安装 / 完成」那颗按钮。

    按钮文案里带助记符（`&Next`），而且随安装器语言变，所以去掉 `&` 之后再比。
    """
    wanted = {"next", "install", "finish", "下一步", "安装", "完成"}
    clicked = []

    def cb(h, _):
        c = ctypes.create_unicode_buffer(96)
        user32.GetClassNameW(h, c, 96)
        t = ctypes.create_unicode_buffer(128)
        user32.GetWindowTextW(h, t, 128)
        label = t.value.replace("&", "").strip().lower()
        if c.value == "TNewButton" and label in wanted and user32.IsWindowEnabled(h):
            user32.SendMessageW(h, BM_CLICK, 0, 0)
            clicked.append(label)
            return False
        return True

    user32.EnumChildWindows(wiz, EnumProc(cb), 0)
    return bool(clicked)


def drive_wizard(timeout=90.0) -> bool:
    """一路点「下一步」把向导走完（等同人工点完整轮安装）。"""
    deadline = time.time() + timeout
    while time.time() < deadline:
        wiz = find_wizard()
        if not wiz:
            return True  # 向导没了 = 装完了
        click_forward(wiz)
        time.sleep(0.7)
    return False


def main() -> int:
    setup, app_exe, choice = sys.argv[1], sys.argv[2], sys.argv[3]

    runner = subprocess.Popen([app_exe])
    time.sleep(5)
    if not alive(runner.pid):
        print("FAIL 前置条件不满足：程序没跑起来")
        return 1
    print(f"程序已在运行 pid={runner.pid}")

    setup_proc = subprocess.Popen([setup])
    hwnd, title, body = find_prompt()
    if not hwnd:
        print(f"FAIL 安装器没有弹出确认框（安装器还活着={alive(setup_proc.pid)}）")
        setup_proc.kill()
        runner.kill()
        return 1

    print(f"确认框: 标题={title!r}")
    print(f"  正文: {body!r}")

    user32.SendMessageW(hwnd, WM_COMMAND, IDYES if choice == "yes" else IDNO, 0)
    time.sleep(3)

    if choice == "yes":
        # 点「是」之后安装继续走向导，这里替人把剩下的「下一步」点完
        drive_wizard()
        deadline = time.time() + 30
        while setup_proc.poll() is None and time.time() < deadline:
            time.sleep(0.3)

    setup_done = setup_proc.poll() is not None
    app_gone = not alive(runner.pid)
    if choice == "yes":
        ok = app_gone and setup_done
        print(f"点「是」后: 程序已退出={app_gone} 安装已走完={setup_done}  ← 都应为 True")
    else:
        ok = not app_gone and setup_done
        print(f"点「否」后: 程序仍在运行={not app_gone} 安装器已退出={setup_done}  ← 都应为 True")

    if alive(runner.pid):
        runner.kill()
    if setup_proc.poll() is None:
        setup_proc.kill()
    print("OK" if ok else "FAIL")
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())

