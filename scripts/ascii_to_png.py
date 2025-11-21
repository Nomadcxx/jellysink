#!/usr/bin/env python3
"""
Convert jellysink ASCII art to PNG with transparent background.

Requirements:
    pip install pillow

Usage:
    python3 scripts/ascii_to_png.py [output_path]

Example:
    python3 scripts/ascii_to_png.py assets/jellysink-header.png

Note: This script is for generating the README header image.
      The ASCII art is stored in internal/ui/ascii.go
"""

from PIL import Image, ImageDraw, ImageFont
import sys

# ASCII art (from internal/ui/ascii.go)
ASCII_ART = """  ████              ████   ████                            ████                ████
                    ████   ████                                                ████
  ████  ██████████  ████   ████  ████  ████  ██████████  ██████    ██████████  ████  ████
  ████  ████  ████  ████   ████  ████  ████  ██████      ██████    ██████████  ██████████
  ████  ██████████  ████   ████  ████  ████  ██████████    ████    ████  ████  ████████
  ████  ████        ████   ████  ██████████      ██████  ████████  ████  ████  ██████████
  ████  ██████████  ████   ████  ██████████  ██████████  ████████  ████  ████  ████  ████
██████                                 ████
██████                           ██████████                                              """

# RAMA theme colors
RAMA_RED = "#ef233c"  # Pantone red

def create_ascii_png(output_path="assets/jellysink-header.png"):
    """Create PNG from ASCII art with transparent background."""

    # Calculate image dimensions
    lines = ASCII_ART.split('\n')
    max_width = max(len(line) for line in lines)
    height = len(lines)

    # Font settings - using monospace
    font_size = 14
    char_width = 8  # Approximate monospace character width
    char_height = 16  # Approximate line height

    # Image dimensions with padding
    img_width = max_width * char_width + 40  # 20px padding on each side
    img_height = height * char_height + 40    # 20px padding top/bottom

    # Create image with transparent background (RGBA)
    img = Image.new('RGBA', (img_width, img_height), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)

    # Try to use a monospace font
    try:
        # Try common monospace fonts
        for font_name in ['/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf',
                          '/System/Library/Fonts/Monaco.dfont',
                          '/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf']:
            try:
                font = ImageFont.truetype(font_name, font_size)
                break
            except:
                continue
        else:
            # Fallback to default font
            font = ImageFont.load_default()
    except:
        font = ImageFont.load_default()

    # Convert hex color to RGB tuple
    color = tuple(int(RAMA_RED.lstrip('#')[i:i+2], 16) for i in (0, 2, 4))

    # Draw text line by line
    y = 20  # Starting y position (top padding)
    for line in lines:
        x = 20  # Starting x position (left padding)
        draw.text((x, y), line, fill=color + (255,), font=font)  # Add alpha channel
        y += char_height

    # Save the image
    img.save(output_path, 'PNG')
    print(f"Created {output_path}")
    print(f"Dimensions: {img_width}x{img_height}")

if __name__ == "__main__":
    output = sys.argv[1] if len(sys.argv) > 1 else "assets/jellysink-header.png"
    create_ascii_png(output)
