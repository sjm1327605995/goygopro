/**
 * Card-face texture generation (P5-4). The card face is the real card art
 * (pics/<code>.jpg, loaded through the field's cardImageProvider), matching the
 * original image_manager.cpp GetImage/GetTexture (image_manager.cpp:215-241)
 * which loads the picture file directly as the face. While the art is in
 * flight — or when no art is installed — the face is a neutral dark placeholder
 * (the original uses textures/unknown.jpg as its tUnknown fallback).
 * Extracted from field3d.ts; takes the field instance as `self` and only does
 * `import type`, so there is no runtime import cycle.
 */
import * as THREE from '../../libs/three.module.js';
import type { DuelField3D } from './field3d.ts';

export function generateCardTexture(self: DuelField3D, cardCode: number): any {
  if (self.cardTextureCache.has(cardCode)) {
    return self.cardTextureCache.get(cardCode);
  }

  const canvas = document.createElement('canvas');
  canvas.width = 512;
  canvas.height = 744;
  const ctx = canvas.getContext('2d')!;

  // 加载中 / 缺图占位：整张卡直接铺满卡图即可，无需程序自绘卡名/攻防/星数
  // （那不是原版行为，原版卡面就是整张卡图）。
  ctx.fillStyle = '#151a24';
  ctx.fillRect(0, 0, canvas.width, canvas.height);

  const texture = new (THREE as any).CanvasTexture(canvas);
  self.cardTextureCache.set(cardCode, texture);

  // Real card art layers in asynchronously — the card is playable immediately
  // either way. pics/<code>.jpg is a complete card face, so it is cover-fit
  // across the whole canvas (512x744 ≈ 59x86mm, distortion negligible).
  if (self.cardImageProvider) {
    self.cardImageProvider(cardCode).then((pic) => {
      if (!pic || !pic.url) return;
      const img = new Image();
      img.onload = () => {
        // A later mesh for the same code may have reused/recreated the socket;
        // only repaint the canvas that still backs the cached texture.
        const tex = self.cardTextureCache.get(cardCode);
        if (!tex || tex.image !== canvas) return;
        const c = canvas.getContext('2d')!;
        c.fillStyle = '#000000';
        c.fillRect(0, 0, canvas.width, canvas.height);
        const scale = Math.max(canvas.width / img.width, canvas.height / img.height);
        const dw = img.width * scale, dh = img.height * scale;
        c.drawImage(img, (canvas.width - dw) / 2, (canvas.height - dh) / 2, dw, dh);
        tex.needsUpdate = true;
      };
      img.src = pic.url;
    }).catch(() => {});
  }
  return texture;
}