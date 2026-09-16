"""Remove the near-uniform ivory matte from this project's illustrations.

Pillow + NumPy only; no image model. Estimate the edge background, keep white
clothing separate by its blue channel, then estimate alpha at silhouette edges
against nearby foreground colors. Also clears enclosed ivory gaps (e.g. a lens).
Only suitable for this illustration set, not general-purpose photo segmentation.
"""
from pathlib import Path
import argparse
import json
import numpy as np
from PIL import Image, ImageFilter


def cutout(source: Path, png: Path, webp: Path):
    image = Image.open(source).convert('RGB')
    rgb = np.asarray(image).astype(np.float32)
    edges = np.concatenate([rgb[:8].reshape(-1, 3), rgb[-8:].reshape(-1, 3),
                            rgb[:, :8].reshape(-1, 3), rgb[:, -8:].reshape(-1, 3)])
    background = np.median(edges, axis=0)
    difference = np.max(np.abs(rgb - background), axis=2)
    # Ivory differs from intentional white clothing by 15–25 in blue.
    background_mask = difference <= 7
    foreground_mask = ~background_mask
    core = np.asarray(Image.fromarray((foreground_mask * 255).astype('uint8'))
                      .filter(ImageFilter.MinFilter(5))) > 0
    # Preserve thin dark contour strokes even when narrower than the erosion.
    core |= np.max(rgb, axis=2) < 105
    known = core.copy()
    nearest = rgb.copy()
    for _ in range(5):
        added = np.zeros_like(known)
        for dy, dx in ((0, 1), (0, -1), (1, 0), (-1, 0), (1, 1), (-1, -1)):
            available = np.roll(known, (dy, dx), axis=(0, 1))
            if dy > 0: available[:dy] = False
            if dy < 0: available[dy:] = False
            if dx > 0: available[:, :dx] = False
            if dx < 0: available[:, dx:] = False
            take = available & ~known & ~added
            values = np.roll(nearest, (dy, dx), axis=(0, 1))
            nearest[take] = values[take]
            added |= take
        known |= added
    alpha = np.ones(difference.shape, dtype=np.float32)
    alpha[background_mask] = 0
    boundary = foreground_mask & ~core & known
    direction = nearest - background
    denominator = np.maximum(np.sum(direction * direction, axis=2), 1)
    estimate = np.clip(np.sum((rgb - background) * direction, axis=2) / denominator, 0, 1)
    alpha[boundary] = estimate[boundary]
    # Remove residual matte from partially covered edge pixels.
    unmatte = np.clip((rgb - (1 - alpha[..., None]) * background) /
                     np.maximum(alpha[..., None], 0.02), 0, 255)
    unmatte[alpha == 0] = 0
    rgba = np.dstack([unmatte.round().astype('uint8'), (alpha * 255).round().astype('uint8')])
    result = Image.fromarray(rgba, 'RGBA')
    png.parent.mkdir(parents=True, exist_ok=True)
    webp.parent.mkdir(parents=True, exist_ok=True)
    result.save(png, optimize=True)
    result.save(webp, format='WEBP', quality=94, method=6, exact=True)
    return {'source': source.name, 'png': str(png), 'webp': str(webp),
            'background_rgb': background.tolist(), 'size': result.size,
            'transparent_pixels': int(np.count_nonzero(alpha == 0)),
            'partial_pixels': int(np.count_nonzero((alpha > 0) & (alpha < 1))),
            'opaque_pixels': int(np.count_nonzero(alpha == 1))}


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('source', type=Path)
    parser.add_argument('png', type=Path)
    parser.add_argument('webp', type=Path)
    args = parser.parse_args()
    print(json.dumps(cutout(args.source, args.png, args.webp)))
