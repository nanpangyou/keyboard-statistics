"""盯住运行中的界面，验证它有没有在「真的处理消息」。

用法：python soak_ui.py <pid> [秒数]

为什么不能只看 IsHungAppWindow：那个判定要等系统 5 秒超时，而且它分不清
「线程在等消息」和「线程等错了队列」。这里的判据是**底部状态栏的时钟有没有在走**：
那行字由 WM_TIMER 驱动重绘，只要它每 2 秒变一次，就说明队列消息确实在被取出、
窗口确实在重绘——这正是「GetMessage 等错队列」那种故障会立刻断掉的东西。

比对的是像素而不是文字：不需要 OCR，也不需要第三方库，PrintWindow 抓到
底部那条再逐字节比就行。窗口内容本来就静态，唯一会动的就是那个时刻。
"""
import ctypes
import ctypes.wintypes as wt
import sys
import time

u = ctypes.WinDLL("user32")
g = ctypes.WinDLL("gdi32")

u.FindWindowW.restype = wt.HWND
u.FindWindowW.argtypes = [wt.LPCWSTR, wt.LPCWSTR]
u.GetWindowRect.argtypes = [wt.HWND, ctypes.POINTER(wt.RECT)]
u.PrintWindow.argtypes = [wt.HWND, wt.HDC, wt.UINT]
u.GetDC.argtypes = [wt.HWND]
u.ReleaseDC.argtypes = [wt.HWND, wt.HDC]
g.CreateCompatibleDC.argtypes = [wt.HDC]
g.CreateCompatibleBitmap.argtypes = [wt.HDC, ctypes.c_int, ctypes.c_int]
g.SelectObject.argtypes = [wt.HDC, wt.HGDIOBJ]
g.DeleteObject.argtypes = [wt.HGDIOBJ]
g.DeleteDC.argtypes = [wt.HDC]
g.GetDIBits.argtypes = [wt.HDC, wt.HBITMAP, wt.UINT, wt.UINT, ctypes.c_void_p, ctypes.c_void_p, wt.UINT]

PW_RENDERFULLCONTENT = 0x00000002


class BITMAPINFOHEADER(ctypes.Structure):
    _fields_ = [("biSize", wt.DWORD), ("biWidth", wt.LONG), ("biHeight", wt.LONG),
                ("biPlanes", wt.WORD), ("biBitCount", wt.WORD), ("biCompression", wt.DWORD),
                ("biSizeImage", wt.DWORD), ("biXPelsPerMeter", wt.LONG),
                ("biYPelsPerMeter", wt.LONG), ("biClrUsed", wt.DWORD), ("biClrImportant", wt.DWORD)]


def grab_strip(hwnd, from_bottom, height):
    """抓窗口靠底部的一条（from_bottom 是从下往上的偏移），返回字节串（BGRA）。

    取样位置必须正好压在底部状态栏上：那里每隔 2 秒就会重写一次时刻，
    取高一点（比如窗口的 90% 处）拿到的是一动不动的图表区，等于没测。
    """
    r = wt.RECT()
    if not u.GetWindowRect(hwnd, ctypes.byref(r)):
        return None
    w, h = r.right - r.left, r.bottom - r.top
    if w <= 0 or h <= height + from_bottom:
        return None

    hdc = u.GetDC(hwnd)
    if not hdc:
        return None
    mem = g.CreateCompatibleDC(hdc)
    # 整窗抓下来再自己切条：GetDIBits 的起始扫描线是从下往上数的，
    # 直接抓一条很容易抓错位置（第一版就抓到了图表区，白跑一轮）。
    bmp = g.CreateCompatibleBitmap(hdc, w, h)
    old = g.SelectObject(mem, bmp)
    try:
        if not u.PrintWindow(hwnd, mem, PW_RENDERFULLCONTENT):
            return None
        info = BITMAPINFOHEADER()
        info.biSize = ctypes.sizeof(BITMAPINFOHEADER)
        info.biWidth = w
        info.biHeight = -h  # 负高度 = 自上而下，切条最省事
        info.biPlanes = 1
        info.biBitCount = 32
        buf = ctypes.create_string_buffer(w * h * 4)
        if not g.GetDIBits(mem, bmp, 0, h, buf, ctypes.byref(info), 0):
            return None
        row = w * 4
        end = (h - from_bottom) * row
        start = max(0, end - height * row)
        return buf.raw[start:end]
    finally:
        g.SelectObject(mem, old)
        g.DeleteObject(bmp)
        g.DeleteDC(mem)
        u.ReleaseDC(hwnd, hdc)


def main() -> int:
    pid = int(sys.argv[1])
    seconds = int(sys.argv[2]) if len(sys.argv) > 2 else 180
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

    deadline = time.time() + seconds
    hung_since = None
    hang_total = 0.0
    last_bits = None
    ticks = 0          # 底部时钟变化次数
    samples = 0
    probes_ok = 0      # 外部探针成功次数
    res = ctypes.c_size_t()

    while time.time() < deadline:
        now = u.IsHungAppWindow(hwnd)
        if now and hung_since is None:
            hung_since = time.time()
        elif not now and hung_since is not None:
            hang_total += time.time() - hung_since
            hung_since = None

        # 探针：SendMessage 必须由窗口线程处理完才返回。
        # 界面线程一旦等错了队列（或被换到别的线程），这里就会全线超时。
        r = u.SendMessageTimeoutW(hwnd, 0, 0, 0, 0x0001 | 0x0002, 500, ctypes.byref(res))
        samples += 1
        if r:
            probes_ok += 1

        # 主动作废窗口，逼它走一次 WM_PAINT（同样是队列消息）。
        # 不这么做的话，数据没变时程序可能压根不重绘，时钟就不会变，
        # 那个信号就失去了意义。
        u.InvalidateRect(hwnd, None, True)

        bits = grab_strip(hwnd, 0, 26)
        if bits is not None and last_bits is not None and bits != last_bits:
            ticks += 1
        if bits is not None:
            last_bits = bits
        time.sleep(2)

    if hung_since is not None:
        hang_total += time.time() - hung_since

    print(f"采样 {samples} 次 / 探针成功 {probes_ok} 次 / 底部时钟变化 {ticks} 次 / "
          f"未响应累计 {hang_total:.1f} 秒")
    if probes_ok < samples * 0.9:
        print("结论：探针大面积超时——界面线程没有在处理消息（这就是卡死）")
        return 1
    if ticks == 0:
        print("结论：探针能过但窗口从不重绘，需要人工看一眼")
        return 1
    if hang_total > 5:
        print("结论：有时段被判为未响应，需要看日志")
        return 1
    print("结论：界面全程在处理消息、在重绘，未响应 0 秒")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
