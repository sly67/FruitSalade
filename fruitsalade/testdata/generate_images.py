#!/usr/bin/env python3
"""Generate test JPEG images with diverse EXIF metadata for gallery testing.

Creates 20 small colored JPEG images with embedded EXIF data covering
different cameras, GPS locations, dates, and shooting parameters.

Requires: Pillow (pip install Pillow)

Usage: python3 generate_images.py
Output: photos/ directory with 20 JPEG files
"""

import os
import struct
import math
from datetime import datetime
from PIL import Image, ImageDraw

OUTPUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "photos")
WIDTH, HEIGHT = 640, 480

# --- EXIF binary builder (minimal TIFF/EXIF via piexif-free approach) ---

# TIFF tag IDs
TAG_IMAGE_WIDTH = 0x0100
TAG_IMAGE_HEIGHT = 0x0101
TAG_MAKE = 0x010F
TAG_MODEL = 0x0110
TAG_ORIENTATION = 0x0112
TAG_EXIF_IFD = 0x8769
TAG_GPS_IFD = 0x8825

# EXIF sub-IFD tags
TAG_EXPOSURE_TIME = 0x829A
TAG_FNUMBER = 0x829D
TAG_ISO = 0x8827
TAG_EXIF_VERSION = 0x9000
TAG_DATE_ORIGINAL = 0x9003
TAG_FOCAL_LENGTH = 0x920A
TAG_FLASH = 0x9209
TAG_LENS_MODEL = 0xA434

# GPS sub-IFD tags
TAG_GPS_LAT_REF = 0x0001
TAG_GPS_LAT = 0x0002
TAG_GPS_LON_REF = 0x0003
TAG_GPS_LON = 0x0004
TAG_GPS_ALT_REF = 0x0005
TAG_GPS_ALT = 0x0006

# TIFF types
BYTE = 1
ASCII = 2
SHORT = 3
LONG = 4
RATIONAL = 5
UNDEFINED = 7


def _pack_rational(num, den):
    return struct.pack("<II", int(num), int(den))


def _deg_to_dms_rational(deg):
    """Convert decimal degrees to (degrees, minutes, seconds) as rationals."""
    d = int(abs(deg))
    m = int((abs(deg) - d) * 60)
    s = (abs(deg) - d - m / 60) * 3600
    # Store seconds as rational with 100 denominator for precision
    return (
        struct.pack("<II", d, 1)
        + struct.pack("<II", m, 1)
        + struct.pack("<II", int(s * 100), 100)
    )


def build_exif(meta):
    """Build a minimal valid EXIF APP1 segment as bytes."""

    # We'll build: APP1 marker + TIFF header + IFD0 + EXIF sub-IFD + GPS sub-IFD
    # All offsets are from TIFF header start (after "Exif\x00\x00")

    ifd0_entries = []
    exif_entries = []
    gps_entries = []
    data_area = bytearray()  # extra data referenced by offset

    def current_data_offset_placeholder():
        """Will be resolved later."""
        return len(data_area)

    def add_string_tag(tag_list, tag_id, value):
        """Add an ASCII string tag."""
        val = value.encode("ascii") + b"\x00"
        tag_list.append((tag_id, ASCII, len(val), val))

    def add_short_tag(tag_list, tag_id, value):
        tag_list.append((tag_id, SHORT, 1, struct.pack("<H", value)))

    def add_long_tag(tag_list, tag_id, value):
        tag_list.append((tag_id, LONG, 1, struct.pack("<I", value)))

    def add_rational_tag(tag_list, tag_id, num, den):
        tag_list.append((tag_id, RATIONAL, 1, _pack_rational(num, den)))

    # IFD0 tags
    add_short_tag(ifd0_entries, TAG_IMAGE_WIDTH, WIDTH)
    add_short_tag(ifd0_entries, TAG_IMAGE_HEIGHT, HEIGHT)
    add_string_tag(ifd0_entries, TAG_MAKE, meta["make"])
    add_string_tag(ifd0_entries, TAG_MODEL, meta["model"])
    add_short_tag(ifd0_entries, TAG_ORIENTATION, meta.get("orientation", 1))
    # Placeholders for EXIF IFD and GPS IFD offsets (filled later)
    ifd0_entries.append((TAG_EXIF_IFD, LONG, 1, None))  # placeholder
    if meta.get("lat") is not None:
        ifd0_entries.append((TAG_GPS_IFD, LONG, 1, None))  # placeholder

    # EXIF sub-IFD tags
    dt = meta["date"]
    date_str = dt.strftime("%Y:%m:%d %H:%M:%S")
    add_string_tag(exif_entries, TAG_DATE_ORIGINAL, date_str)
    add_rational_tag(exif_entries, TAG_FOCAL_LENGTH, int(meta["focal_length"] * 10), 10)
    add_rational_tag(
        exif_entries, TAG_FNUMBER, int(meta["aperture"] * 10), 10
    )
    # Exposure time as rational
    exp = meta["exposure"]
    if exp < 1:
        add_rational_tag(exif_entries, TAG_EXPOSURE_TIME, 1, int(1 / exp))
    else:
        add_rational_tag(exif_entries, TAG_EXPOSURE_TIME, int(exp), 1)
    add_short_tag(exif_entries, TAG_ISO, meta["iso"])
    add_short_tag(exif_entries, TAG_FLASH, 1 if meta.get("flash") else 0)
    if meta.get("lens"):
        add_string_tag(exif_entries, TAG_LENS_MODEL, meta["lens"])
    # ExifVersion
    exif_entries.append((TAG_EXIF_VERSION, UNDEFINED, 4, b"0232"))

    # GPS sub-IFD tags
    if meta.get("lat") is not None:
        lat, lon = meta["lat"], meta["lon"]
        add_string_tag(gps_entries, TAG_GPS_LAT_REF, "N" if lat >= 0 else "S")
        gps_entries.append((TAG_GPS_LAT, RATIONAL, 3, _deg_to_dms_rational(lat)))
        add_string_tag(gps_entries, TAG_GPS_LON_REF, "E" if lon >= 0 else "W")
        gps_entries.append((TAG_GPS_LON, RATIONAL, 3, _deg_to_dms_rational(lon)))
        gps_entries.append((TAG_GPS_ALT_REF, BYTE, 1, b"\x00"))
        alt = meta.get("altitude", 0)
        add_rational_tag(gps_entries, TAG_GPS_ALT, int(alt * 10), 10)

    # Now serialize everything
    # TIFF header: "II" (little-endian) + 0x002A + offset to IFD0 (8)
    tiff_header = b"II" + struct.pack("<HI", 0x002A, 8)

    def serialize_ifd(entries, base_offset):
        """Serialize an IFD. Returns (ifd_bytes, data_bytes, entry_offsets_to_patch)."""
        num = len(entries)
        ifd = struct.pack("<H", num)
        extra = bytearray()
        # Data that doesn't fit in 4 bytes goes after the IFD
        # IFD size: 2 + num*12 + 4 (next IFD pointer)
        ifd_size = 2 + num * 12 + 4
        data_start = base_offset + ifd_size

        for tag_id, dtype, count, value in entries:
            if value is None:
                # Placeholder - will be patched
                ifd += struct.pack("<HHII", tag_id, dtype, count, 0)
                continue

            val_bytes = value
            if isinstance(val_bytes, int):
                val_bytes = struct.pack("<I", val_bytes)

            # Size per type
            type_sizes = {BYTE: 1, ASCII: 1, SHORT: 2, LONG: 4, RATIONAL: 8, UNDEFINED: 1}
            total_size = count * type_sizes.get(dtype, 1)

            if total_size <= 4:
                # Inline value (pad to 4 bytes)
                padded = val_bytes[:4].ljust(4, b"\x00")
                ifd += struct.pack("<HHI", tag_id, dtype, count) + padded
            else:
                # Offset to data area
                offset = data_start + len(extra)
                ifd += struct.pack("<HHII", tag_id, dtype, count, offset)
                extra += val_bytes

        # Next IFD pointer = 0 (no next IFD)
        ifd += struct.pack("<I", 0)
        return ifd, bytes(extra)

    # Calculate offsets
    # TIFF header = 8 bytes
    # IFD0 starts at offset 8
    ifd0_count = len(ifd0_entries)
    ifd0_size = 2 + ifd0_count * 12 + 4

    # Serialize IFD0 first to know its data size
    ifd0_bytes, ifd0_data = serialize_ifd(ifd0_entries, 8)

    # EXIF sub-IFD starts after IFD0 + its data
    exif_offset = 8 + len(ifd0_bytes) + len(ifd0_data)
    exif_bytes, exif_data = serialize_ifd(exif_entries, exif_offset)

    # GPS sub-IFD starts after EXIF sub-IFD + its data
    gps_offset = exif_offset + len(exif_bytes) + len(exif_data)
    if gps_entries:
        gps_bytes, gps_data = serialize_ifd(gps_entries, gps_offset)
    else:
        gps_bytes, gps_data = b"", b""

    # Patch the EXIF IFD offset in IFD0
    # Find the TAG_EXIF_IFD entry and patch its value
    ifd0_patched = bytearray(ifd0_bytes)
    entry_start = 2  # skip count
    for i in range(ifd0_count):
        pos = entry_start + i * 12
        tag = struct.unpack_from("<H", ifd0_patched, pos)[0]
        if tag == TAG_EXIF_IFD:
            struct.pack_into("<I", ifd0_patched, pos + 8, exif_offset)
        elif tag == TAG_GPS_IFD:
            struct.pack_into("<I", ifd0_patched, pos + 8, gps_offset)

    # Assemble TIFF data
    tiff = tiff_header + bytes(ifd0_patched) + ifd0_data + exif_bytes + exif_data
    if gps_entries:
        tiff += gps_bytes + gps_data

    # Wrap in APP1 segment: FF E1 + length + "Exif\x00\x00" + TIFF
    exif_payload = b"Exif\x00\x00" + tiff
    app1_length = len(exif_payload) + 2  # +2 for length field itself
    app1 = b"\xFF\xE1" + struct.pack(">H", app1_length) + exif_payload

    return app1


def create_gradient_image(color1, color2, pattern="horizontal"):
    """Create a gradient image between two RGB colors."""
    img = Image.new("RGB", (WIDTH, HEIGHT))
    draw = ImageDraw.Draw(img)

    r1, g1, b1 = color1
    r2, g2, b2 = color2

    if pattern == "horizontal":
        for x in range(WIDTH):
            t = x / WIDTH
            r = int(r1 + (r2 - r1) * t)
            g = int(g1 + (g2 - g1) * t)
            b = int(b1 + (b2 - b1) * t)
            draw.line([(x, 0), (x, HEIGHT)], fill=(r, g, b))
    elif pattern == "vertical":
        for y in range(HEIGHT):
            t = y / HEIGHT
            r = int(r1 + (r2 - r1) * t)
            g = int(g1 + (g2 - g1) * t)
            b = int(b1 + (b2 - b1) * t)
            draw.line([(0, y), (WIDTH, y)], fill=(r, g, b))
    elif pattern == "diagonal":
        for y in range(HEIGHT):
            for x in range(WIDTH):
                t = (x / WIDTH + y / HEIGHT) / 2
                r = int(r1 + (r2 - r1) * t)
                g = int(g1 + (g2 - g1) * t)
                b = int(b1 + (b2 - b1) * t)
                img.putpixel((x, y), (r, g, b))
    elif pattern == "radial":
        cx, cy = WIDTH // 2, HEIGHT // 2
        max_dist = math.sqrt(cx * cx + cy * cy)
        for y in range(HEIGHT):
            for x in range(WIDTH):
                dist = math.sqrt((x - cx) ** 2 + (y - cy) ** 2)
                t = min(dist / max_dist, 1.0)
                r = int(r1 + (r2 - r1) * t)
                g = int(g1 + (g2 - g1) * t)
                b = int(b1 + (b2 - b1) * t)
                img.putpixel((x, y), (r, g, b))

    return img


def save_jpeg_with_exif(img, filepath, exif_app1):
    """Save JPEG and inject EXIF APP1 segment after SOI marker."""
    import io

    # Save to buffer without EXIF
    buf = io.BytesIO()
    img.save(buf, format="JPEG", quality=85)
    jpeg_data = buf.getvalue()

    # JPEG starts with FF D8 (SOI). Insert APP1 right after.
    with open(filepath, "wb") as f:
        f.write(jpeg_data[:2])  # SOI
        f.write(exif_app1)  # APP1 EXIF
        f.write(jpeg_data[2:])  # rest of JPEG


# --- Image definitions ---

IMAGES = [
    # Paris Vacation (blue/violet gradients)
    {
        "name": "paris_01.jpg",
        "colors": ((30, 30, 120), (100, 60, 180)),
        "pattern": "horizontal",
        "make": "Canon",
        "model": "Canon EOS R5",
        "lens": "RF 24-70mm F2.8L IS USM",
        "focal_length": 35,
        "aperture": 2.8,
        "exposure": 1 / 250,
        "iso": 200,
        "flash": False,
        "date": datetime(2024, 6, 15, 10, 30, 0),
        "lat": 48.8584,
        "lon": 2.2945,
        "altitude": 35,
        "orientation": 1,
    },
    {
        "name": "paris_02.jpg",
        "colors": ((40, 20, 140), (120, 80, 200)),
        "pattern": "vertical",
        "make": "Canon",
        "model": "Canon EOS R5",
        "lens": "RF 24-70mm F2.8L IS USM",
        "focal_length": 50,
        "aperture": 4.0,
        "exposure": 1 / 500,
        "iso": 100,
        "flash": False,
        "date": datetime(2024, 6, 15, 14, 45, 0),
        "lat": 48.8606,
        "lon": 2.3376,
        "altitude": 40,
        "orientation": 1,
    },
    {
        "name": "paris_03.jpg",
        "colors": ((50, 30, 100), (90, 50, 160)),
        "pattern": "diagonal",
        "make": "Canon",
        "model": "Canon EOS R5",
        "lens": "RF 70-200mm F2.8L IS USM",
        "focal_length": 135,
        "aperture": 2.8,
        "exposure": 1 / 1000,
        "iso": 400,
        "flash": False,
        "date": datetime(2024, 6, 16, 9, 15, 0),
        "lat": 48.8530,
        "lon": 2.3499,
        "altitude": 45,
        "orientation": 1,
    },
    {
        "name": "paris_04.jpg",
        "colors": ((20, 10, 90), (80, 40, 170)),
        "pattern": "horizontal",
        "make": "Canon",
        "model": "Canon EOS R5",
        "lens": "RF 24-70mm F2.8L IS USM",
        "focal_length": 24,
        "aperture": 8.0,
        "exposure": 1 / 125,
        "iso": 100,
        "flash": False,
        "date": datetime(2024, 6, 17, 18, 0, 0),
        "lat": 48.8520,
        "lon": 2.3510,
        "altitude": 30,
        "orientation": 1,
    },
    {
        "name": "paris_05.jpg",
        "colors": ((60, 40, 150), (140, 100, 220)),
        "pattern": "radial",
        "make": "Canon",
        "model": "Canon EOS R5",
        "lens": "RF 24-70mm F2.8L IS USM",
        "focal_length": 70,
        "aperture": 5.6,
        "exposure": 1 / 60,
        "iso": 800,
        "flash": True,
        "date": datetime(2024, 6, 17, 21, 30, 0),
        "lat": 48.8566,
        "lon": 2.3522,
        "altitude": 38,
        "orientation": 1,
    },
    # Tokyo Trip (red/orange gradients)
    {
        "name": "tokyo_01.jpg",
        "colors": ((180, 40, 20), (220, 120, 30)),
        "pattern": "horizontal",
        "make": "Sony",
        "model": "ILCE-7M4",
        "lens": "FE 24-105mm F4 G OSS",
        "focal_length": 24,
        "aperture": 4.0,
        "exposure": 1 / 500,
        "iso": 200,
        "flash": False,
        "date": datetime(2024, 9, 3, 8, 0, 0),
        "lat": 35.6762,
        "lon": 139.6503,
        "altitude": 15,
        "orientation": 1,
    },
    {
        "name": "tokyo_02.jpg",
        "colors": ((200, 60, 10), (240, 140, 40)),
        "pattern": "vertical",
        "make": "Sony",
        "model": "ILCE-7M4",
        "lens": "FE 24-105mm F4 G OSS",
        "focal_length": 70,
        "aperture": 4.0,
        "exposure": 1 / 250,
        "iso": 400,
        "flash": False,
        "date": datetime(2024, 9, 4, 12, 30, 0),
        "lat": 35.7148,
        "lon": 139.7967,
        "altitude": 20,
        "orientation": 1,
    },
    {
        "name": "tokyo_03.jpg",
        "colors": ((160, 30, 30), (200, 100, 20)),
        "pattern": "diagonal",
        "make": "Sony",
        "model": "ILCE-7M4",
        "lens": "FE 85mm F1.4 GM",
        "focal_length": 85,
        "aperture": 1.4,
        "exposure": 1 / 2000,
        "iso": 100,
        "flash": False,
        "date": datetime(2024, 9, 5, 17, 45, 0),
        "lat": 35.6595,
        "lon": 139.7004,
        "altitude": 10,
        "orientation": 6,
    },
    {
        "name": "tokyo_04.jpg",
        "colors": ((190, 50, 15), (230, 130, 50)),
        "pattern": "radial",
        "make": "Sony",
        "model": "ILCE-7M4",
        "lens": "FE 24-105mm F4 G OSS",
        "focal_length": 105,
        "aperture": 5.6,
        "exposure": 1 / 30,
        "iso": 1600,
        "flash": True,
        "date": datetime(2024, 9, 5, 22, 0, 0),
        "lat": 35.6938,
        "lon": 139.7035,
        "altitude": 5,
        "orientation": 1,
    },
    # Nature Hike (green gradients)
    {
        "name": "nature_01.jpg",
        "colors": ((20, 100, 30), (60, 180, 60)),
        "pattern": "vertical",
        "make": "Nikon",
        "model": "NIKON Z 6III",
        "lens": "NIKKOR Z 24-120mm f/4 S",
        "focal_length": 24,
        "aperture": 8.0,
        "exposure": 1 / 250,
        "iso": 200,
        "flash": False,
        "date": datetime(2025, 3, 10, 7, 30, 0),
        "lat": 37.7456,
        "lon": -119.5936,
        "altitude": 1200,
        "orientation": 1,
    },
    {
        "name": "nature_02.jpg",
        "colors": ((30, 120, 20), (80, 200, 50)),
        "pattern": "horizontal",
        "make": "Nikon",
        "model": "NIKON Z 6III",
        "lens": "NIKKOR Z 24-120mm f/4 S",
        "focal_length": 70,
        "aperture": 5.6,
        "exposure": 1 / 500,
        "iso": 100,
        "flash": False,
        "date": datetime(2025, 3, 10, 10, 0, 0),
        "lat": 37.7490,
        "lon": -119.5884,
        "altitude": 1350,
        "orientation": 1,
    },
    {
        "name": "nature_03.jpg",
        "colors": ((10, 80, 40), (50, 160, 70)),
        "pattern": "radial",
        "make": "Nikon",
        "model": "NIKON Z 6III",
        "lens": "NIKKOR Z 100-400mm f/4.5-5.6 VR S",
        "focal_length": 300,
        "aperture": 5.6,
        "exposure": 1 / 1000,
        "iso": 800,
        "flash": False,
        "date": datetime(2025, 3, 11, 15, 20, 0),
        "lat": 37.7400,
        "lon": -119.6000,
        "altitude": 1100,
        "orientation": 1,
    },
    {
        "name": "nature_04.jpg",
        "colors": ((40, 140, 25), (90, 210, 55)),
        "pattern": "diagonal",
        "make": "Nikon",
        "model": "NIKON Z 6III",
        "lens": "NIKKOR Z 24-120mm f/4 S",
        "focal_length": 50,
        "aperture": 4.0,
        "exposure": 1 / 125,
        "iso": 400,
        "flash": False,
        "date": datetime(2025, 3, 11, 17, 45, 0),
        "lat": 37.7420,
        "lon": -119.5950,
        "altitude": 1250,
        "orientation": 1,
    },
    # Family Portraits (warm tones)
    {
        "name": "family_01.jpg",
        "colors": ((200, 150, 100), (240, 190, 140)),
        "pattern": "radial",
        "make": "Apple",
        "model": "iPhone 15 Pro",
        "lens": "iPhone 15 Pro back triple camera 6.765mm f/1.78",
        "focal_length": 6.8,
        "aperture": 1.8,
        "exposure": 1 / 120,
        "iso": 50,
        "flash": False,
        "date": datetime(2025, 1, 1, 12, 0, 0),
        "lat": 40.7580,
        "lon": -73.9855,
        "altitude": 25,
        "orientation": 1,
    },
    {
        "name": "family_02.jpg",
        "colors": ((210, 160, 110), (245, 200, 150)),
        "pattern": "horizontal",
        "make": "Apple",
        "model": "iPhone 15 Pro",
        "lens": "iPhone 15 Pro back triple camera 6.765mm f/1.78",
        "focal_length": 6.8,
        "aperture": 1.8,
        "exposure": 1 / 60,
        "iso": 200,
        "flash": True,
        "date": datetime(2025, 1, 1, 19, 30, 0),
        "lat": 40.7484,
        "lon": -73.9857,
        "altitude": 20,
        "orientation": 6,
    },
    {
        "name": "family_03.jpg",
        "colors": ((190, 140, 90), (235, 185, 130)),
        "pattern": "vertical",
        "make": "Apple",
        "model": "iPhone 15 Pro",
        "lens": "iPhone 15 Pro back triple camera 6.765mm f/1.78",
        "focal_length": 6.8,
        "aperture": 2.2,
        "exposure": 1 / 250,
        "iso": 64,
        "flash": False,
        "date": datetime(2025, 1, 2, 10, 15, 0),
        "lat": 40.6892,
        "lon": -74.0445,
        "altitude": 10,
        "orientation": 1,
    },
    # Macro/Studio (abstract patterns, no GPS)
    {
        "name": "macro_01.jpg",
        "colors": ((100, 20, 80), (200, 60, 160)),
        "pattern": "radial",
        "make": "Fujifilm",
        "model": "X-T5",
        "lens": "XF 80mm F2.8 R LM OIS WR Macro",
        "focal_length": 80,
        "aperture": 5.6,
        "exposure": 1 / 125,
        "iso": 400,
        "flash": True,
        "date": datetime(2023, 11, 5, 14, 0, 0),
        "lat": None,
        "lon": None,
        "orientation": 1,
    },
    {
        "name": "macro_02.jpg",
        "colors": ((120, 30, 90), (220, 80, 180)),
        "pattern": "diagonal",
        "make": "Fujifilm",
        "model": "X-T5",
        "lens": "XF 80mm F2.8 R LM OIS WR Macro",
        "focal_length": 80,
        "aperture": 8.0,
        "exposure": 1 / 60,
        "iso": 200,
        "flash": True,
        "date": datetime(2023, 11, 5, 15, 30, 0),
        "lat": None,
        "lon": None,
        "orientation": 1,
    },
    {
        "name": "macro_03.jpg",
        "colors": ((80, 10, 70), (180, 50, 150)),
        "pattern": "horizontal",
        "make": "Fujifilm",
        "model": "X-T5",
        "lens": "XF 80mm F2.8 R LM OIS WR Macro",
        "focal_length": 80,
        "aperture": 2.8,
        "exposure": 1 / 500,
        "iso": 800,
        "flash": False,
        "date": datetime(2023, 11, 12, 10, 0, 0),
        "lat": None,
        "lon": None,
        "orientation": 1,
    },
    {
        "name": "macro_04.jpg",
        "colors": ((140, 40, 100), (230, 90, 190)),
        "pattern": "vertical",
        "make": "Fujifilm",
        "model": "X-T5",
        "lens": "XF 80mm F2.8 R LM OIS WR Macro",
        "focal_length": 80,
        "aperture": 4.0,
        "exposure": 1 / 250,
        "iso": 320,
        "flash": True,
        "date": datetime(2023, 11, 12, 11, 45, 0),
        "lat": None,
        "lon": None,
        "orientation": 1,
    },
]


def main():
    os.makedirs(OUTPUT_DIR, exist_ok=True)

    for meta in IMAGES:
        img = create_gradient_image(meta["colors"][0], meta["colors"][1], meta["pattern"])

        # Add a text label on the image for visual identification
        draw = ImageDraw.Draw(img)
        label = meta["name"].replace(".jpg", "").replace("_", " ").title()
        # Simple text at top-left
        draw.text((10, 10), label, fill=(255, 255, 255))
        draw.text((10, 30), f'{meta["make"]} {meta["model"]}', fill=(200, 200, 200))
        draw.text((10, 50), meta["date"].strftime("%Y-%m-%d %H:%M"), fill=(200, 200, 200))

        exif_data = build_exif(meta)
        filepath = os.path.join(OUTPUT_DIR, meta["name"])
        save_jpeg_with_exif(img, filepath, exif_data)
        print(f"  {meta['name']} ({len(open(filepath,'rb').read())} bytes)")

    print(f"\nGenerated {len(IMAGES)} images in {OUTPUT_DIR}")


if __name__ == "__main__":
    main()
