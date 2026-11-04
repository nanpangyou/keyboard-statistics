"""量「敲键到界面数字跳动」的延迟，用来确认刷新节奏真的够快。

用法：python refresh_latency.py <数据目录> [轮数]

做法：程序开着不动，脚本往它的库里加一批计数（就像是刚敲了键），
然后每 100ms 抓一次窗口上半部分的像素，等它变；变了就是界面已经跟上。
**底部状态栏要排除掉**：那里每秒都在变，会盖住真正要观测的变化。

为什么用像素而不是读程序内部状态：从外面只能看到界面，用户能感到的也正是这个。
这也是唯一能验证「刷新间隔从 2 秒改到 500ms」的办法——那个常量在进程里，
没有对外接口。

注意：只对**测试用**的数据目录用（会往库里写假数据），别指向用户的真实数据。
"""
import ctypes
import ctypes.wintypes as wt
import datetime as dt
import os
import sqlite3
import sys
import time

u = ctypes.WinDLL("user32")
g = ctypes.WinDLL("gdi32")

u.FindWindowW.restype = wt.HWND
u.FindWindowW.argtypes = [wt.LPCWSTR, wt.LPCWSTR]
u.GetWindowRect.argtypes = [wt.HWND, ctypes.POINTER(wt.RECT)]
u.GetClientRect.argtypes = [wt.HWND, ctypes.POINTER(wt.RECT)]
u.PrintWindow.argtypes = [wt.HWND, wt.HDC, wt.UINT]
u.GetDC.argtypes = [wt.HWND]
u.ReleaseDC.argtypes = [wt.HWND, wt.HDC]
g.CreateCompatibleDC.argtypes = [wt.HDC]
g.CreateCompatibleBitmap.argtypes = [wt.HDC, ctypes.c_int, ctypes.c_int]
g.SelectObject.argtypes = [wt.HDC, wt.HGDIOBJ]
g.DeleteObject.argtypes = [wt.HGDIOBJ]
g.DeleteDC.argtypes = [wt.HDC]
g.GetDIBits.argtypes = [wt.HDC, wt.HBITMAP, wt.UINT, wt.UINT, ctypes.c_void_p,
                        ctypes.c_void_p, wt.UINT]

PW_RENDERFULLCONTENT = 0x00000002
FOOTER_H = 40  # 底部这么高的区域排除掉（状态栏每秒都在变）


class BMIH(ctypes.Structure):
    _fields_ = [("biSize", wt.DWORD), ("biWidth", wt.LONG), ("biHeight", wt.LONG),
                ("biPlanes", wt.WORD), ("biBitCount", wt.WORD), ("biCompression", wt.DWORD),
                ("biSizeImage", wt.DWORD), ("biXPelsPerMeter", wt.LONG),
                ("biYPelsPerMeter", wt.LONG), ("biClrUsed", wt.DWORD), ("biClrImportant", wt.DWORD)]


def grab(hwnd):
    """抓窗口像素（不含底部状态栏），返回字节串。"""
    wr, cr = wt.RECT(), wt.RECT()
    u.GetWindowRect(hwnd, ctypes.byref(wr))
    u.GetClientRect(hwnd, ctypes.byref(cr))
    w, h = wr.right - wr.left, wr.bottom - wr.top
    if w <= 0 or h <= FOOTER_H:
        return None
    hdc = u.GetDC(hwnd)
    mem = g.CreateCompatibleDC(hdc)
    bmp = g.CreateCompatibleBitmap(hdc, w, h)
    old = g.SelectObject(mem, bmp)
    try:
        u.PrintWindow(hwnd, mem, PW_RENDERFULLCONTENT)
        info = BMIH()
        info.biSize = ctypes.sizeof(BMIH)
        info.biWidth, info.biHeight = w, -h
        info.biPlanes, info.biBitCount = 1, 32
        buf = ctypes.create_string_buffer(w * h * 4)
        g.GetDIBits(mem, bmp, 0, h, buf, ctypes.byref(info), 0)
        row = w * 4
        return buf.raw[: (h - FOOTER_H) * row]
    finally:
        g.SelectObject(mem, old)
        g.DeleteObject(bmp)
        g.DeleteDC(mem)
        u.ReleaseDC(hwnd, hdc)


def bump(datadir, amount):
    """往今天的计数里加 amount 次，模拟「刚敲了一片键」。"""
    db = os.path.join(datadir, "stats.db")
    day = dt.date.today().isoformat()
    hour = dt.datetime.now().hour
    con = sqlite3.connect(db, timeout=5)
    try:
        con.execute(
            "INSERT INTO key_count (day, hour, vk, count) VALUES (?, ?, 0x20, ?) "
            "ON CONFLICT(day, hour, vk) DO UPDATE SET count = count + excluded.count",
            (day, hour, amount))
        con.commit()
    finally:
        con.close()


def main() -> int:
    datadir = sys.argv[1]
    rounds = int(sys.argv[2]) if len(sys.argv) > 2 else 3
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

    hwnd = 0
    for _ in range(60):
        hwnd = u.FindWindowW(None, "键盘使用统计")
        if hwnd:
            break
        time.sleep(0.5)
    if not hwnd:
        print("找不到窗口")
        return 2

    lat = []
    for i in range(rounds):
        base = grab(hwnd)
        if base is None:
            print("抓不到窗口像素")
            return 2
        time.sleep(0.3)  # 让上一批改动先消化掉
        base = grab(hwnd)
        started = time.time()
        bump(datadir, 137)
        waited = None
        while time.time() - started < 6:
            cur = grab(hwnd)
            if cur is not None and cur != base:
                waited = time.time() - started
                break
            time.sleep(0.1)
        if waited is None:
            print(f"第 {i + 1} 轮：6 秒内界面没反应（超时）")
            return 1
        lat.append(waited)
        print(f"第 {i + 1} 轮：数字在 {waited * 1000:.0f} ms 后变了")
        time.sleep(0.6)

    lat.sort()
    mid = lat[len(lat) // 2]
    print(f"中位延迟 {mid * 1000:.0f} ms（{', '.join(f'{x * 1000:.0f}' for x in lat)}）")
    if mid > 1.0:
        print("结论：超过 1 秒，用户会感到「统计有延迟」")
        return 1
    print("结论：半秒级，符合「敲完就动」的预期")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
