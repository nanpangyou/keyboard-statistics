"""一条命令跑完：启动 -> 收进托盘 -> 发握手消息 -> 记录退出时间线。

用法：python handshake_timeline.py <exe路径> <数据目录>
"""
import ctypes
import ctypes.wintypes as wt
import subprocess
import sys
import time

user32 = ctypes.WinDLL("user32", use_last_error=True)
kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)

MAIN_TITLE = "键盘使用统计"
QUIT_MSG_NAME = "KeyboardStatistics.QuitForUpgrade"
WM_CLOSE = 0x0010

user32.FindWindowW.restype = wt.HWND
user32.FindWindowW.argtypes = [wt.LPCWSTR, wt.LPCWSTR]
user32.GetWindowThreadProcessId.argtypes = [wt.HWND, ctypes.POINTER(wt.DWORD)]
user32.PostMessageW.argtypes = [wt.HWND, wt.UINT, wt.WPARAM, wt.LPARAM]
user32.RegisterWindowMessageW.restype = wt.UINT
user32.RegisterWindowMessageW.argtypes = [wt.LPCWSTR]
kernel32.OpenProcess.restype = wt.HANDLE


def owner_of(hwnd) -> int:
    p = wt.DWORD()
    user32.GetWindowThreadProcessId(hwnd, ctypes.byref(p))
    return p.value


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
    proc = subprocess.Popen([exe], cwd=datadir)
    pid = proc.pid
    print(f"启动 pid={pid}")

    hwnd = 0
    for _ in range(50):
        time.sleep(0.2)
        hwnd = user32.FindWindowW(None, MAIN_TITLE)
        if hwnd:
            break
    if not hwnd:
        print("FAIL 主窗口没出现")
        proc.kill()
        return 1
    print(f"主窗口 hwnd={hwnd} 归属 pid={owner_of(hwnd)} (期望 {pid})")

    # 收进托盘
    user32.PostMessageW(hwnd, WM_CLOSE, 0, 0)
    time.sleep(1.0)
    print(f"收进托盘后 存活={alive(pid)} 窗口句柄仍存在={bool(user32.FindWindowW(None, MAIN_TITLE))}")

    # 握手消息
    msg = user32.RegisterWindowMessageW(QUIT_MSG_NAME)
    t0 = time.time()
    user32.PostMessageW(hwnd, msg, 0, 0)
    print(f"已发送握手消息 id={msg}")

    exit_t = win_t = 0.0
    for _ in range(150):  # 最多 15 秒
        time.sleep(0.1)
        w = user32.FindWindowW(None, MAIN_TITLE)
        now = time.time() - t0
        if not exit_t and not alive(pid):
            exit_t = now
        if not win_t and not w:
            win_t = now
        if exit_t and win_t:
            break

    print(f"进程退出于 {exit_t:.2f}s；窗口消失于 {win_t:.2f}s（都应为秒级，远快于 RM 的 30 秒）")
    ok = 0 < exit_t < 5 and 0 < win_t < 5
    print("OK" if ok else "FAIL")
    if alive(pid):
        proc.kill()
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
