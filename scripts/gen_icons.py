"""生成 fnUPnP 应用图标（橙色渐变圆角 + 白色路由器/端口转发图形）。

产出：
- fpk/ICON.PNG        64x64   应用商店图标
- fpk/ICON_256.PNG    256x256 应用商店大图标
- fpk/app/ui/images/icon_64.png
- fpk/app/ui/images/icon_256.png
"""
import os

from PIL import Image, ImageDraw

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
FPK = os.path.join(ROOT, "fpk")
UI_IMAGES = os.path.join(FPK, "app", "ui", "images")

TOP = (255, 176, 77)      # #ffb04d
BOTTOM = (245, 107, 0)    # #f56b00
ORANGE = (245, 107, 0)
WHITE = (255, 255, 255, 255)


def _gradient(size):
    base = Image.new("RGB", (size, size))
    d = ImageDraw.Draw(base)
    for y in range(size):
        t = y / (size - 1)
        r = int(TOP[0] + (BOTTOM[0] - TOP[0]) * t)
        g = int(TOP[1] + (BOTTOM[1] - TOP[1]) * t)
        b = int(TOP[2] + (BOTTOM[2] - TOP[2]) * t)
        d.line([(0, y), (size, y)], fill=(r, g, b))
    return base


def make_icon(size):
    base = _gradient(size)

    mask = Image.new("L", (size, size), 0)
    ImageDraw.Draw(mask).rounded_rectangle(
        [0, 0, size - 1, size - 1], radius=int(size * 0.22), fill=255
    )

    icon = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    icon.paste(base, (0, 0), mask)

    d = ImageDraw.Draw(icon)
    s = size

    # 天线
    antenna_w = max(2, int(s * 0.045))
    for cx in (0.38, 0.62):
        x = cx * s
        d.rounded_rectangle(
            [x - antenna_w / 2, 0.20 * s, x + antenna_w / 2, 0.34 * s],
            radius=antenna_w // 2,
            fill=WHITE,
        )

    # 路由器机身
    d.rounded_rectangle(
        [0.24 * s, 0.36 * s, 0.76 * s, 0.68 * s],
        radius=int(s * 0.06),
        fill=WHITE,
    )

    # 端口转发箭头
    cy = 0.52 * s
    ah = 0.05 * s
    x0 = 0.33 * s
    x1 = 0.60 * s
    d.rectangle([x0, cy - ah, x1, cy + ah], fill=ORANGE)
    d.polygon(
        [(x1, cy - ah * 2.0), (x1 + 0.13 * s, cy), (x1, cy + ah * 2.0)],
        fill=ORANGE,
    )

    return icon


def main():
    os.makedirs(UI_IMAGES, exist_ok=True)

    big = make_icon(256)
    small = big.resize((64, 64), Image.LANCZOS)

    big.save(os.path.join(FPK, "ICON_256.PNG"))
    small.save(os.path.join(FPK, "ICON.PNG"))
    big.save(os.path.join(UI_IMAGES, "icon_256.png"))
    small.save(os.path.join(UI_IMAGES, "icon_64.png"))

    print("icons generated:")
    for p in (os.path.join(FPK, "ICON.PNG"), os.path.join(FPK, "ICON_256.PNG"),
              os.path.join(UI_IMAGES, "icon_64.png"), os.path.join(UI_IMAGES, "icon_256.png")):
        print("  ", os.path.relpath(p, ROOT))


if __name__ == "__main__":
    main()
