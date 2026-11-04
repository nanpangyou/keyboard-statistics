"""把设计稿 PNG 转成多尺寸 app.ico（exe 图标 + 托盘图标共用）。

用法：python scripts/make-icon.py <源图.png> [输出路径]

做四件事：
  1. 按 alpha 通道裁掉四周空白，再留一点边距——设计稿四周常有大量透明区域，
     直接缩放会让图形在 16x16 里小得看不清；
  2. 生成 Windows 需要的整组尺寸（16/20/24/32/40/48/64/128/256）；
  3. **小尺寸（≤24）不走缩放，而是按同一母题现画一遍**：设计稿里有折线、滑杆、
     旋钮、四色垫，缩到 16px 全糊成一团灰。托盘里那个 16px 图标就只能看到脏点，
     所以小尺寸单独画「面板 + 绿色折线」——形状简单、边是直的，在 16px 下仍然是硬的；
  4. 写成一个多尺寸 .ico。

只依赖 Pillow。
"""
import sys

from PIL import Image, ImageDraw

SIZES = [16, 20, 24, 32, 40, 48, 64, 128, 256]
PADDING_RATIO = 0.04  # 图形四周留 4% 空白，避免贴边

# 小尺寸现画用的配色，取自设计稿：面板浅灰、描边深蓝、折线绿。
SMALL_ICON_MAX = 24
COL_PLATE = (231, 229, 228, 255)
COL_EDGE = (43, 58, 85, 255)
COL_WAVE = (61, 190, 107, 255)
COL_PAD = (255, 179, 0, 255)


def draw_small(size: int) -> Image.Image:
    """按「面板 + 绿色折线」现画一个小尺寸图标。

    为什么不缩放：16px 下折线只剩一两个像素宽，抗锯齿之后是一片半透明的灰，
    在深色任务栏上几乎看不见。现画就能保证描边正好 1px、折线正好 2px，
    每个像素都是实色，托盘里才清爽。
    """
    s = size
    im = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)

    edge = max(1, round(s / 16))          # 边框粗细：16px 时 1px
    margin = max(1, round(s * 0.07))      # 四周留白
    radius = max(2, round(s * 0.22))      # 圆角

    # 面板：浅灰底 + 深蓝描边（描边保证深色/浅色任务栏上都看得见边界）
    d.rounded_rectangle(
        [margin, margin, s - 1 - margin, s - 1 - margin],
        radius=radius, fill=COL_PLATE, outline=COL_EDGE, width=edge,
    )

    # 绿色折线：低—高—低—中，四个点让 16px 下也能看出「有起伏」而不是一道横线
    left = margin + max(1, round(s * 0.12))
    right = s - 1 - margin - max(1, round(s * 0.12))
    top = margin + round(s * 0.26)
    bottom = s - 1 - margin - round(s * 0.26)
    span = bottom - top
    xs = [left, left + (right - left) * 0.34, left + (right - left) * 0.66, right]
    ys = [top + span * 0.62, top + span * 0.10, top + span * 0.92, top + span * 0.38]
    d.line(list(zip(xs, ys)), fill=COL_WAVE, width=max(2, round(s / 7)), joint="curve")

    # 右上角一小块暖色：呼应设计稿里那排彩色垫，也避免整枚图标只有灰和绿
    if s >= 20:
        pad_size = max(2, round(s * 0.17))
        px0 = s - 1 - margin - pad_size - max(1, round(s * 0.06))
        py0 = margin + max(1, round(s * 0.06))
        d.rounded_rectangle([px0, py0, px0 + pad_size, py0 + pad_size],
                            radius=max(1, pad_size // 3), fill=COL_PAD)

    return im


def main() -> int:
    src = sys.argv[1] if len(sys.argv) > 1 else "assets/app.png"
    dst = sys.argv[2] if len(sys.argv) > 2 else "assets/app.ico"

    im = Image.open(src).convert("RGBA")

    bbox = im.getbbox()  # 非透明内容的包围盒
    if bbox:
        im = im.crop(bbox)
        print(f"裁掉透明边: {bbox} -> {im.size}")

    side = max(im.size)
    pad = int(side * PADDING_RATIO)
    canvas = Image.new("RGBA", (side + pad * 2, side + pad * 2), (0, 0, 0, 0))
    canvas.paste(im, ((canvas.width - im.width) // 2, (canvas.height - im.height) // 2))

    frames = []
    for s in SIZES:
        if s <= SMALL_ICON_MAX:
            frames.append(draw_small(s))
        else:
            frames.append(canvas.resize((s, s), Image.LANCZOS))

    frames[-1].save(dst, format="ICO", sizes=[(s, s) for s in SIZES], append_images=frames[:-1])
    drawn = [s for s in SIZES if s <= SMALL_ICON_MAX]
    print(f"写出 {dst}: {', '.join(f'{s}x{s}' for s in SIZES)}")
    print(f"  其中 {', '.join(f'{s}x{s}' for s in drawn)} 是按母题现画的（不缩放）")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
