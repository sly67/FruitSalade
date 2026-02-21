#!/usr/bin/env python3
"""Generate conflict test files for testing the Conflicts UI.

Creates:
- Image conflict variants (shifted colors) for a few photos
- WAV audio files (3 seconds) and their conflicts
- MP4 video files (5 seconds, via ffmpeg) and their conflicts
- Valid PDF documents with actual page content and their conflicts

Requires: Pillow (pip install Pillow), ffmpeg

Usage: python3 generate_conflicts.py
"""

import io
import os
import struct
import subprocess
import math
import zlib
from PIL import Image, ImageDraw

BASE_DIR = os.path.dirname(os.path.abspath(__file__))
PHOTOS_DIR = os.path.join(BASE_DIR, "photos")
MEDIA_DIR = os.path.join(BASE_DIR, "media")

WIDTH, HEIGHT = 640, 480


def create_gradient_image(color1, color2, pattern="horizontal", w=WIDTH, h=HEIGHT):
    """Create a gradient image between two RGB colors."""
    img = Image.new("RGB", (w, h))
    draw = ImageDraw.Draw(img)
    r1, g1, b1 = color1
    r2, g2, b2 = color2

    if pattern == "horizontal":
        for x in range(w):
            t = x / w
            r = int(r1 + (r2 - r1) * t)
            g = int(g1 + (g2 - g1) * t)
            b = int(b1 + (b2 - b1) * t)
            draw.line([(x, 0), (x, h)], fill=(r, g, b))
    elif pattern == "vertical":
        for y in range(h):
            t = y / h
            r = int(r1 + (r2 - r1) * t)
            g = int(g1 + (g2 - g1) * t)
            b = int(b1 + (b2 - b1) * t)
            draw.line([(0, y), (w, y)], fill=(r, g, b))
    elif pattern == "radial":
        cx, cy = w // 2, h // 2
        max_dist = math.sqrt(cx * cx + cy * cy)
        for y in range(h):
            for x in range(w):
                dist = math.sqrt((x - cx) ** 2 + (y - cy) ** 2)
                t = min(dist / max_dist, 1.0)
                r = int(r1 + (r2 - r1) * t)
                g = int(g1 + (g2 - g1) * t)
                b = int(b1 + (b2 - b1) * t)
                img.putpixel((x, y), (r, g, b))
    return img


# ─── WAV Generator ───────────────────────────────────────────────────────────

def generate_wav(filepath, freq=440, duration_ms=3000, sample_rate=22050):
    """Generate a WAV file with a sine tone."""
    num_samples = int(sample_rate * duration_ms / 1000)
    samples = bytearray()
    for i in range(num_samples):
        t = i / sample_rate
        val = int(16000 * math.sin(2 * math.pi * freq * t))
        samples += struct.pack("<h", max(-32768, min(32767, val)))

    data_size = len(samples)
    file_size = 36 + data_size

    with open(filepath, "wb") as f:
        f.write(b"RIFF")
        f.write(struct.pack("<I", file_size))
        f.write(b"WAVE")
        f.write(b"fmt ")
        f.write(struct.pack("<I", 16))
        f.write(struct.pack("<H", 1))            # PCM
        f.write(struct.pack("<H", 1))            # mono
        f.write(struct.pack("<I", sample_rate))
        f.write(struct.pack("<I", sample_rate * 2))
        f.write(struct.pack("<H", 2))            # block align
        f.write(struct.pack("<H", 16))           # bits/sample
        f.write(b"data")
        f.write(struct.pack("<I", data_size))
        f.write(samples)


# ─── Video Generator (ffmpeg) ────────────────────────────────────────────────

def generate_mp4(filepath, color_hex, label, duration=5):
    """Generate an MP4 video using ffmpeg with a color background and text."""
    subprocess.run([
        "ffmpeg", "-y",
        "-f", "lavfi",
        "-i", f"color=c={color_hex}:size=640x480:duration={duration}:rate=24",
        "-vf", f"drawtext=text='{label}':fontsize=28:fontcolor=white:x=20:y=20,"
               f"drawtext=text='%{{pts\\:hms}}':fontsize=20:fontcolor=white:x=20:y=440",
        "-c:v", "libx264",
        "-preset", "ultrafast",
        "-pix_fmt", "yuv420p",
        "-movflags", "+faststart",
        filepath,
    ], check=True, capture_output=True)


# ─── PDF Generator ───────────────────────────────────────────────────────────

def generate_pdf(filepath, title, body_lines, accent_color="0 0 0.8"):
    """Generate a valid multi-page PDF with text content."""
    objects = {}
    obj_id = [0]

    def new_obj():
        obj_id[0] += 1
        return obj_id[0]

    # Build page content streams
    pages_content = []
    lines_per_page = 35
    for page_start in range(0, len(body_lines), lines_per_page):
        page_lines = body_lines[page_start:page_start + lines_per_page]
        stream = "BT\n"
        stream += "/F1 14 Tf\n"
        stream += f"{accent_color} rg\n"
        stream += "50 750 Td\n"
        stream += f"({_pdf_escape(title)}) Tj\n"
        stream += "0 0 0 rg\n"
        stream += "/F1 10 Tf\n"
        stream += "0 -24 Td\n"
        for line in page_lines:
            stream += f"({_pdf_escape(line)}) Tj\n"
            stream += "0 -14 Td\n"
        stream += "ET\n"
        pages_content.append(stream)

    catalog_id = new_obj()
    pages_id = new_obj()
    font_id = new_obj()

    page_ids = []
    for content in pages_content:
        content_id = new_obj()
        page_id = new_obj()
        compressed = zlib.compress(content.encode("latin-1"))
        objects[content_id] = (
            f"<< /Length {len(compressed)} /Filter /FlateDecode >>\n"
            f"stream\n"
        ).encode("ascii") + compressed + b"\nendstream"
        objects[page_id] = (
            f"<< /Type /Page /Parent {pages_id} 0 R "
            f"/MediaBox [0 0 612 792] "
            f"/Contents {content_id} 0 R "
            f"/Resources << /Font << /F1 {font_id} 0 R >> >> >>"
        ).encode("ascii")
        page_ids.append(page_id)

    objects[font_id] = b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"

    kids = " ".join(f"{pid} 0 R" for pid in page_ids)
    objects[pages_id] = (
        f"<< /Type /Pages /Kids [{kids}] /Count {len(page_ids)} >>"
    ).encode("ascii")

    objects[catalog_id] = (
        f"<< /Type /Catalog /Pages {pages_id} 0 R >>"
    ).encode("ascii")

    pdf = bytearray(b"%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
    offsets = {}

    for oid in sorted(objects.keys()):
        offsets[oid] = len(pdf)
        pdf += f"{oid} 0 obj\n".encode("ascii")
        pdf += objects[oid]
        pdf += b"\nendobj\n"

    xref_offset = len(pdf)
    pdf += b"xref\n"
    pdf += f"0 {obj_id[0] + 1}\n".encode("ascii")
    pdf += b"0000000000 65535 f \n"
    for oid in range(1, obj_id[0] + 1):
        pdf += f"{offsets[oid]:010d} 00000 n \n".encode("ascii")

    pdf += b"trailer\n"
    pdf += f"<< /Size {obj_id[0] + 1} /Root {catalog_id} 0 R >>\n".encode("ascii")
    pdf += b"startxref\n"
    pdf += f"{xref_offset}\n".encode("ascii")
    pdf += b"%%EOF\n"

    with open(filepath, "wb") as f:
        f.write(pdf)


def _pdf_escape(text):
    return text.replace("\\", "\\\\").replace("(", "\\(").replace(")", "\\)")


# ─── Definitions ─────────────────────────────────────────────────────────────

IMAGE_CONFLICTS = [
    {
        "original": "paris_01.jpg",
        "conflict_date": "2026-02-19",
        "colors": ((40, 40, 140), (110, 70, 190)),
        "pattern": "horizontal",
        "label": "Paris 01 (edited on laptop)",
    },
    {
        "original": "nature_02.jpg",
        "conflict_date": "2026-02-18",
        "colors": ((40, 130, 30), (90, 210, 60)),
        "pattern": "horizontal",
        "label": "Nature 02 (cropped version)",
    },
    {
        "original": "family_01.jpg",
        "conflict_date": "2026-02-20",
        "colors": ((210, 160, 110), (250, 200, 150)),
        "pattern": "radial",
        "label": "Family 01 (brightness adjusted)",
    },
]


def main():
    os.makedirs(PHOTOS_DIR, exist_ok=True)
    os.makedirs(MEDIA_DIR, exist_ok=True)
    count = 0

    # --- Image conflicts ---
    for ic in IMAGE_CONFLICTS:
        orig = ic["original"]
        base, ext = os.path.splitext(orig)
        conflict_name = f"{base} (conflict {ic['conflict_date']}){ext}"
        filepath = os.path.join(PHOTOS_DIR, conflict_name)
        img = create_gradient_image(ic["colors"][0], ic["colors"][1], ic["pattern"])
        draw = ImageDraw.Draw(img)
        draw.text((10, 10), ic["label"], fill=(255, 255, 255))
        img.save(filepath, format="JPEG", quality=85)
        print(f"  {conflict_name} ({os.path.getsize(filepath)} bytes)")
        count += 1

    # --- Audio ---
    wav_orig = os.path.join(MEDIA_DIR, "recording.wav")
    generate_wav(wav_orig, freq=440, duration_ms=3000)
    print(f"  media/recording.wav ({os.path.getsize(wav_orig)} bytes)")
    count += 1

    wav_conf = os.path.join(MEDIA_DIR, "recording (conflict 2026-02-20).wav")
    generate_wav(wav_conf, freq=523, duration_ms=3500)
    print(f"  media/recording (conflict 2026-02-20).wav ({os.path.getsize(wav_conf)} bytes)")
    count += 1

    # --- Video (MP4 via ffmpeg, 5 seconds each) ---
    vid_orig = os.path.join(MEDIA_DIR, "clip.mp4")
    generate_mp4(vid_orig, "#1E1E96", "Original Clip", duration=5)
    print(f"  media/clip.mp4 ({os.path.getsize(vid_orig)} bytes)")
    count += 1

    vid_conf = os.path.join(MEDIA_DIR, "clip (conflict 2026-02-19).mp4")
    generate_mp4(vid_conf, "#961E1E", "Conflict Clip", duration=5)
    print(f"  media/clip (conflict 2026-02-19).mp4 ({os.path.getsize(vid_conf)} bytes)")
    count += 1

    # --- PDF ---
    generate_pdf(
        os.path.join(MEDIA_DIR, "report.pdf"),
        "FruitSalade - Quarterly Report",
        [
            "Summary",
            "",
            "This quarter saw significant progress on the FruitSalade platform.",
            "Key milestones include the completion of multi-backend storage,",
            "RBAC-based group permissions, and the new web application.",
            "",
            "Infrastructure",
            "",
            "- PostgreSQL metadata store with 7 migrations",
            "- S3, Local, and SMB storage backends via Router",
            "- Docker deployment with embedded PostgreSQL",
            "- Prometheus metrics and Grafana dashboard",
            "",
            "Client Features",
            "",
            "- FUSE client with on-demand file hydration",
            "- LRU cache with configurable size and pinning",
            "- SSE-based real-time sync notifications",
            "- Conflict detection and version history",
            "",
            "Security",
            "",
            "- TLS 1.3 with automatic certificate management",
            "- JWT authentication with token rotation",
            "- OIDC integration for enterprise SSO",
            "- Per-user rate limiting and quotas",
        ],
        accent_color="0 0.2 0.6",
    )
    print(f"  media/report.pdf ({os.path.getsize(os.path.join(MEDIA_DIR, 'report.pdf'))} bytes)")
    count += 1

    generate_pdf(
        os.path.join(MEDIA_DIR, "report (conflict 2026-02-18).pdf"),
        "FruitSalade - Quarterly Report (Draft v2)",
        [
            "Summary",
            "",
            "This quarter saw significant progress on the FruitSalade platform.",
            "Key milestones include the completion of multi-backend storage,",
            "RBAC-based group permissions, and the new web application.",
            "",
            "Infrastructure",
            "",
            "- PostgreSQL metadata store with 7 migrations",
            "- S3, Local, and SMB storage backends via Router",
            "- Docker deployment with embedded PostgreSQL",
            "- Prometheus metrics and Grafana dashboard",
            "- NEW: CI/CD pipeline via GitHub Actions",
            "- NEW: Integration test suite with Docker fixtures",
            "",
            "Client Features",
            "",
            "- FUSE client with on-demand file hydration",
            "- LRU cache with configurable size and pinning",
            "- SSE-based real-time sync notifications",
            "- Conflict detection and version history",
            "- NEW: Windows client with CfAPI cloud files",
            "- NEW: Pin/unpin CLI for offline access control",
            "",
            "Security",
            "",
            "- TLS 1.3 with automatic certificate management",
            "- JWT authentication with token rotation",
            "- OIDC integration for enterprise SSO",
            "- Per-user rate limiting and quotas",
            "",
            "Web Application",
            "",
            "- Gallery with tag management and albums",
            "- File browser with drag-and-drop upload",
            "- Admin dashboard with storage locations",
            "- Version history and conflict resolution UI",
        ],
        accent_color="0.6 0.1 0",
    )
    print(f"  media/report (conflict 2026-02-18).pdf ({os.path.getsize(os.path.join(MEDIA_DIR, 'report (conflict 2026-02-18).pdf'))} bytes)")
    count += 1

    # Clean up old files
    for old in ["clip.webm", "clip (conflict 2026-02-19).webm",
                "clip.avi", "clip (conflict 2026-02-19).avi"]:
        old_path = os.path.join(MEDIA_DIR, old)
        if os.path.exists(old_path):
            os.remove(old_path)
            print(f"  removed old {old}")

    print(f"\nGenerated {count} conflict test files")


if __name__ == "__main__":
    main()
