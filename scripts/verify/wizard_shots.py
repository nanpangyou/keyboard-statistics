"""把安装向导开起来，逐页截图，验证界面语言。

用法：python wizard_shots.py <setup.exe> <输出前缀> [最多截图页数]

为什么不用 /VERYSILENT：静默模式根本不画向导，界面语言对不对就无从验证。
这里走正常安装流程，逐页截图，最后点「取消」退出——**不会真的装上**。
（验证打包与升级另做静默安装。）

要点：安装包在工作区里跑会被沙箱按 Low Integrity 处理，必须先生到 %TEMP% 再运行。
"""
import ctypes
import ctypes.wintypes as wt
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path

u = ctypes.WinDLL("user32")
g = ctypes.WinDLL("gdi32")

u.FindWindowExW.restype = wt.HWND
u.FindWindowExW.argtypes = [wt.HWND, wt.HWND, wt.LPCWSTR, wt.LPCWSTR]
u.GetWindowTextW.argtypes = [wt.HWND, wt.LPWSTR, ctypes.c_int]
u.GetWindowRect.argtypes = [wt.HWND, ctypes.POINTER(wt.RECT)]
u.EnumChildWindows.argtypes = [wt.HWND, ctypes.c_void_p, wt.LPARAM]
u.GetClassNameW.argtypes = [wt.HWND, wt.LPWSTR, ctypes.c_int]
u.SendMessageW.argtypes = [wt.HWND, wt.UINT, wt.WPARAM, wt.LPARAM]
u.IsWindowVisible.argtypes = [wt.HWND]
u.GetDC.argtypes = [wt.HWND]
u.ReleaseDC.argtypes = [wt.HWND, wt.HDC]
g.CreateCompatibleDC.argtypes = [wt.HDC]
g.CreateCompatibleBitmap.argtypes = [wt.HDC, ctypes.c_int, ctypes.c_int]
g.SelectObject.argtypes = [wt.HDC, wt.HGDIOBJ]
g.DeleteObject.argtypes = [wt.HGDIOBJ]
g.DeleteDC.argtypes = [wt.HDC]
g.GetDIBits.argtypes = [wt.HDC, wt.HBITMAP, wt.UINT, wt.UINT, ctypes.c_void_p,
                        ctypes.c_void_p, wt.UINT]

WM_CLICK = 0x00F5
BM_CLICK = 0x00F5
PW_RENDERFULLCONTENT = 0x00000002


class BMIH(ctypes.Structure):
    _fields_ = [("biSize", wt.DWORD), ("biWidth", wt.LONG), ("biHeight", wt.LONG),
                ("biPlanes", wt.WORD), ("biBitCount", wt.WORD), ("biCompression", wt.DWORD),
                ("biSizeImage", wt.DWORD), ("biXPelsPerMeter", wt.LONG),
                ("biYPelsPerMeter", wt.LONG), ("biClrUsed", wt.DWORD), ("biClrImportant", wt.DWORD)]


def text_of(hwnd) -> str:
    buf = ctypes.create_unicode_buffer(512)
    u.GetWindowTextW(hwnd, buf, 512)
    return buf.value


def class_of(hwnd) -> str:
    buf = ctypes.create_unicode_buffer(256)
    u.GetClassNameW(hwnd, buf, 256)
    return buf.value


def children(hwnd):
    found = []

    @ctypes.WINFUNCTYPE(wt.BOOL, wt.HWND, wt.LPARAM)
    def cb(h, _):
        found.append(h)
        return True

    u.EnumChildWindows(hwnd, cb, 0)
    return found


def shot(hwnd, path):
    r = wt.RECT()
    u.GetWindowRect(hwnd, ctypes.byref(r))
    w, h = r.right - r.left, r.bottom - r.top
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
        from PIL import Image
        Image.frombuffer("RGBA", (w, h), buf.raw, "raw", "BGRA", 0, 1).convert("RGB").save(path)
    finally:
        g.SelectObject(mem, old)
        g.DeleteObject(bmp)
        g.DeleteDC(mem)
        u.ReleaseDC(hwnd, hdc)


def click_button(wizard, wanted):
    """按文案找按钮并点它（Inno 的按钮是子窗口，发 BM_CLICK 即可）。"""
    for h in children(wizard):
        if class_of(h) != "TNewButton" or not u.IsWindowVisible(h):
            continue
        label = text_of(h).replace("&", "")
        if any(w in label for w in wanted):
            u.SendMessageW(h, BM_CLICK, 0, 0)
            return label
    return ""


def main() -> int:
    src = Path(sys.argv[1])
    prefix = sys.argv[2]
    pages = int(sys.argv[3]) if len(sys.argv) > 3 else 4
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

    stage = Path(tempfile.gettempdir()) / "ks-wizard"
    if stage.exists():
        shutil.rmtree(stage)
    stage.mkdir()
    exe = stage / src.name
    shutil.copy2(src, exe)

    proc = subprocess.Popen([str(exe)])
    # 装了两种语言时，Inno 会先弹「选择安装语言」对话框，**此时向导还没建**，
    # 直接找 TWizardForm 会一直找不到（踩过）。先把这个对话框按掉。
    lang = 0
    for _ in range(60):
        lang = u.FindWindowExW(0, 0, "TSelectLanguageForm", None)
        if lang:
            break
        time.sleep(0.3)
    if lang:
        time.sleep(0.8)
        print("语言选择框标题：", text_of(lang))
        labels = sorted({text_of(h) for h in children(lang)
                         if class_of(h) == "TNewButton" and u.IsWindowVisible(h)})
        print("   按钮：", " | ".join(labels))
        shot(lang, f"{prefix}-0-language.png")
        print("   默认语言（下拉框当前项即中文系统下的自动选择）")
        if not click_button(lang, ["确定", "OK", "是", "Yes"]):
            print("没找到语言框的确定按钮")
            proc.kill()
            return 2

    wizard = 0
    for _ in range(60):
        wizard = u.FindWindowExW(0, 0, "TWizardForm", None)
        if wizard:
            break
        time.sleep(0.3)
    if not wizard:
        print("没等到安装向导窗口")
        return 2

    time.sleep(1.0)
    print("向导标题：", text_of(wizard))

    for i in range(pages):
        title = text_of(wizard)
        out = f"{prefix}-{i + 1}.png"
        shot(wizard, out)
        print(f"第 {i + 1} 页  标题={title!r}  截图={out}")
        # 页面上所有可见按钮文案（用来判断「下一步」是不是中文）
        labels = sorted({text_of(h) for h in children(wizard)
                         if class_of(h) == "TNewButton" and u.IsWindowVisible(h)})
        print("   按钮：", " | ".join(labels))
        clicked = click_button(wizard, ["下一步", "Next"])
        if not clicked:
            print("   没有「下一步」按钮，提前结束")
            break
        time.sleep(1.2)

    # 收尾：点取消，避免真的装上
    canceled = click_button(wizard, ["取消", "Cancel"])
    print("收尾点击：", canceled or "（没找到取消按钮）")
    time.sleep(1.5)
    try:
        proc.wait(timeout=10)
    except subprocess.TimeoutExpired:
        proc.kill()
        print("安装进程没自己退出，已强杀")
    print("进程退出码：", proc.returncode)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
