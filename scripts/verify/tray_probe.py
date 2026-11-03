"""托盘细节验证：隐藏 / 可见性 / 托盘图标是否真的注册在系统里。

mode:
  hide    发 WM_CLOSE，把窗口收进托盘
  visible 打印当前窗口可见性
  probe   用 Shell_NotifyIconW(NIM_MODIFY) 探图标是否注册。
          同时用一个不存在的窗口做对照——如果对照也返回 TRUE，
          说明这个探针没有鉴别力，结论不可用。

用法：python tray_probe.py <pid> <mode>
"""
import ctypes
import ctypes.wintypes as wt
import sys
import time

user32 = ctypes.WinDLL("user32", use_last_error=True)
shell32 = ctypes.WinDLL("shell32", use_last_error=True)

WM_CLOSE = 0x0010
NIM_MODIFY = 1
NIF_TIP = 0x4

user32.FindWindowW.restype = wt.HWND
user32.IsWindowVisible.argtypes = [wt.HWND]
user32.PostMessageW.argtypes = [wt.HWND, wt.UINT, wt.WPARAM, wt.LPARAM]


class NOTIFYICONDATAW(ctypes.Structure):
    _fields_ = [
        ("cbSize", wt.DWORD), ("hWnd", wt.HWND), ("uID", wt.UINT),
        ("uFlags", wt.UINT), ("uCallbackMessage", wt.UINT), ("hIcon", wt.HANDLE),
        ("szTip", wt.WCHAR * 128), ("dwState", wt.DWORD), ("dwStateMask", wt.DWORD),
        ("szInfo", wt.WCHAR * 256), ("uTimeout", wt.UINT),
        ("szInfoTitle", wt.WCHAR * 64), ("dwInfoFlags", wt.DWORD),
        ("guidItem", ctypes.c_byte * 16), ("hBalloonIcon", wt.HANDLE),
    ]


def find(pid: int, title: str) -> int:
    found = []

    @ctypes.WINFUNCTYPE(wt.BOOL, wt.HWND, wt.LPARAM)
    def cb(hwnd, _):
        p = wt.DWORD()
        user32.GetWindowThreadProcessId(hwnd, ctypes.byref(p))
        if p.value == pid:
            buf = ctypes.create_unicode_buffer(256)
            user32.GetWindowTextW(hwnd, buf, 256)
            if buf.value == title:
                found.append(hwnd)
                return False
        return True

    user32.EnumWindows(cb, 0)
    return found[0] if found else 0


def probe(hwnd: int) -> bool:
    nid = NOTIFYICONDATAW()
    nid.cbSize = ctypes.sizeof(NOTIFYICONDATAW)
    nid.hWnd = hwnd
    nid.uID = 1
    nid.uFlags = NIF_TIP
    nid.szTip = "probe"
    return bool(shell32.Shell_NotifyIconW(NIM_MODIFY, ctypes.byref(nid)))


def main() -> int:
    pid = int(sys.argv[1])
    mode = sys.argv[2]
    hwnd = find(pid, "键盘使用统计")
    if hwnd == 0:
        print("FAIL no-window")
        return 1
    title = ctypes.create_unicode_buffer(256)
    cls = ctypes.create_unicode_buffer(256)
    user32.GetWindowTextW(wt.HWND(hwnd), title, 256)
    user32.GetClassNameW(wt.HWND(hwnd), cls, 256)
    print(f"matched hwnd={hwnd} class={cls.value} title={title.value!r} "
          f"visible={bool(user32.IsWindowVisible(wt.HWND(hwnd)))}")

    if mode == "hide":
        user32.PostMessageW(hwnd, WM_CLOSE, 0, 0)
        time.sleep(1.5)
        print(f"hidden={not user32.IsWindowVisible(hwnd)}")
    elif mode == "visible":
        print(f"visible={bool(user32.IsWindowVisible(hwnd))}")
    elif mode == "probe":
        real = probe(hwnd)
        fake = probe(0x00000000)  # 对照：不存在的窗口
        verdict = "OK icon-registered" if real and not fake else "INCONCLUSIVE"
        print(f"real={real} control={fake} {verdict}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
