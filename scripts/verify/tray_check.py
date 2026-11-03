"""托盘常驻的一次性完整验证。

做四件事，全部只看窗口状态和进程是否还活着：
  1. 关掉首启的欢迎弹框（模态框会挡住后面的所有消息测试）
  2. 发 WM_CLOSE —— 窗口收进托盘，进程必须还活着
  3. 发托盘图标的左键回调消息 —— 窗口必须重新出现
  4. 再收起来，交给外部启动第二个实例，验证它能被唤醒

同一套判断也顺带证明了托盘图标确实注册成功：如果 trayInit 失败，
程序会走「没有托盘就真退出」的兜底分支，第 2 步进程就没了。

用法：python tray_check.py <pid> <step>
  step = ready | hide | restore | visible
"""
import ctypes
import ctypes.wintypes as wt
import sys
import time

user32 = ctypes.WinDLL("user32", use_last_error=True)
kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)

WM_CLOSE = 0x0010
WM_COMMAND = 0x0111
IDOK = 1
WM_LBUTTONUP = 0x0202
MSG_TRAY = 0x8002  # 与 internal/ui/win32.go 的 msgTray 一致
MAIN_TITLE = "键盘使用统计"

user32.EnumWindows.argtypes = [ctypes.WINFUNCTYPE(wt.BOOL, wt.HWND, wt.LPARAM), wt.LPARAM]
user32.GetWindowThreadProcessId.argtypes = [wt.HWND, ctypes.POINTER(wt.DWORD)]
user32.GetWindowTextW.argtypes = [wt.HWND, wt.LPWSTR, ctypes.c_int]
user32.GetClassNameW.argtypes = [wt.HWND, wt.LPWSTR, ctypes.c_int]
user32.IsWindowVisible.argtypes = [wt.HWND]
user32.PostMessageW.argtypes = [wt.HWND, wt.UINT, wt.WPARAM, wt.LPARAM]


def windows(pid: int):
    """列出该进程的所有顶层窗口：(hwnd, class, title, visible)。"""
    out = []

    @ctypes.WINFUNCTYPE(wt.BOOL, wt.HWND, wt.LPARAM)
    def cb(hwnd, _):
        p = wt.DWORD()
        user32.GetWindowThreadProcessId(hwnd, ctypes.byref(p))
        if p.value == pid:
            t = ctypes.create_unicode_buffer(256)
            c = ctypes.create_unicode_buffer(256)
            user32.GetWindowTextW(hwnd, t, 256)
            user32.GetClassNameW(hwnd, c, 256)
            out.append((hwnd, c.value, t.value, bool(user32.IsWindowVisible(hwnd))))
        return True

    user32.EnumWindows(cb, 0)
    return out


def main_window(pid: int) -> int:
    for hwnd, _, title, _ in windows(pid):
        if title == MAIN_TITLE:
            return hwnd
    return 0


def dismiss_dialogs(pid: int) -> int:
    """给欢迎弹框的按钮发 BM_CLICK，相当于替用户点了「确定」。

    注意不能用 WM_COMMAND(IDOK)：MessageBox 有自带的模态循环，
    外部投递的 WM_COMMAND 不一定能让它结束（实测关不掉，然后
    后面等它的写库动作就永远等不到）。直接点按钮端口才可靠。
    """
    BM_CLICK = 0x00F5
    dialogs = [(h, c) for h, c, _, _ in windows(pid) if c == "#32770"]

    @ctypes.WINFUNCTYPE(wt.BOOL, wt.HWND, wt.LPARAM)
    def click_child(hwnd, _):
        cls = ctypes.create_unicode_buffer(64)
        user32.GetClassNameW(hwnd, cls, 64)
        if cls.value == "Button":
            user32.PostMessageW(hwnd, BM_CLICK, 0, 0)
            return False
        return True

    for hwnd, _ in dialogs:
        user32.EnumChildWindows(hwnd, click_child, 0)
    return len(dialogs)


def alive(pid: int) -> bool:
    h = kernel32.OpenProcess(0x1000, False, pid)
    if not h:
        return False
    code = wt.DWORD()
    kernel32.GetExitCodeProcess(h, ctypes.byref(code))
    kernel32.CloseHandle(h)
    return code.value == 259  # STILL_ACTIVE


def main() -> int:
    pid = int(sys.argv[1])
    step = sys.argv[2]

    if step == "ready":
        n = dismiss_dialogs(pid)
        time.sleep(0.8)
        hwnd = main_window(pid)
        if hwnd == 0:
            print("FAIL 找不到主窗口")
            return 1
        visible = bool(user32.IsWindowVisible(hwnd))
        print(f"{'OK  ' if visible else 'FAIL'} 关闭弹框 {n} 个后主窗口可见={visible}")
        return 0 if visible else 1

    hwnd = main_window(pid)
    if hwnd == 0:
        print("FAIL 找不到主窗口")
        return 1

    if step == "hide":
        user32.PostMessageW(hwnd, WM_CLOSE, 0, 0)
        time.sleep(1.5)
        vis = bool(user32.IsWindowVisible(hwnd))
        ok = alive(pid) and not vis
        print(f"{'OK  ' if ok else 'FAIL'} 关窗后：进程存活={alive(pid)} 窗口可见={vis}（期望 True/False）")
        return 0 if ok else 1

    if step == "restore":
        user32.PostMessageW(hwnd, MSG_TRAY, 1, WM_LBUTTONUP)
        time.sleep(1.5)
        vis = bool(user32.IsWindowVisible(hwnd))
        print(f"{'OK  ' if vis else 'FAIL'} 托盘左键回调后窗口可见={vis}（期望 True）")
        return 0 if vis else 1

    if step == "visible":
        vis = bool(user32.IsWindowVisible(hwnd))
        print(f"{'OK  ' if vis else 'FAIL'} 窗口可见={vis}")
        return 0 if vis else 1

    print(f"unknown step {step}")
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
