"""把设计稿 PNG 转成多尺寸 app.ico（exe 图标 + 托盘图标共用）。

用法：python scripts/make-icon.py <源图.png> [输出路径]

做三件事：
  1. 按 alpha 通道裁掉四周空白，再留一点边距——设计稿四周常有大量透明区域，
     直接缩放会让图形在 16x16 里小得看不清；
  2. 生成 Windows 需要的整组尺寸（16/20/24/32/40/48/64/128/256）；
  3. 写成一个多尺寸 .ico。

只依赖 Pillow。
"""
import sys

from PIL import Image

SIZES = [16, 20, 24, 32, 40, 48, 64, 128, 256]
PADDING_RATIO = 0.04  # 图形四周留 4% 空白，避免贴边


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

    # Pillow 会自己从最大的图缩出其余尺寸，这里显式逐档缩放以保证小尺寸清晰
    frames = [canvas.resize((s, s), Image.LANCZOS) for s in SIZES]
    frames[-1].save(dst, format="ICO", sizes=[(s, s) for s in SIZES], append_images=frames[:-1])
    print(f"写出 {dst}: {', '.join(f'{s}x{s}' for s in SIZES)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
