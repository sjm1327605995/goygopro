// Port of drawing.cpp DrawSpec cases 1–7. All durations are engine frames.
export type SpecKind = 'flip' | 'zoom' | 'reveal' | 'negated' | 'fade' | 'number';
export interface SpecImages {
  card: CanvasImageSource | null;
  mask: CanvasImageSource | null;
  negated: CanvasImageSource | null;
  number: CanvasImageSource | null;
}
interface Scale { sx: number; sy: number }
type Point = [number, number];

export function cardHintRect(x: number, y: number, x2: number, y2: number, scale: Scale): [number, number, number, number] {
  const mul = Math.min(scale.sx, scale.sy);
  const left = Math.trunc(662 * scale.sx + (x - 662) * mul);
  const top = Math.trunc(277 * scale.sy + (y - 277) * mul);
  return [left, top, Math.trunc(662 * scale.sx + (x2 - 662) * mul) - left,
    Math.trunc(277 * scale.sy + (y2 - 277) * mul) - top];
}

function texturedQuad(ctx: CanvasRenderingContext2D, image: CanvasImageSource, points: Point[]) {
  for (const [a, b, c] of [[0, 1, 2], [0, 2, 3]]) {
    ctx.save();
    ctx.beginPath();
    ctx.moveTo(...points[a]); ctx.lineTo(...points[b]); ctx.lineTo(...points[c]); ctx.closePath(); ctx.clip();
    const [p0, p1, p2, p3] = points;
    if (b === 1) ctx.transform(p1[0] - p0[0], p1[1] - p0[1], p2[0] - p1[0], p2[1] - p1[1], ...p0);
    else ctx.transform(p2[0] - p3[0], p2[1] - p3[1], p3[0] - p0[0], p3[1] - p0[1], ...p0);
    ctx.drawImage(image, 0, 0, 1, 1);
    ctx.restore();
  }
}

export function drawSpecFrame(ctx: CanvasRenderingContext2D, kind: SpecKind, frame: number, images: SpecImages, scale: Scale, num = 1) {
  const rect = (x: number, y: number, x2: number, y2: number) => cardHintRect(x, y, x2, y2, scale);
  const cardRect = rect(574, 150, 751, 404);
  const card = () => { if (images.card) ctx.drawImage(images.card, ...cardRect); };
  ctx.clearRect(0, 0, ctx.canvas.width, ctx.canvas.height);
  if (kind === 'flip') {
    const mul = Math.min(scale.sx, scale.sy);
    const y = Math.sin(Math.min(90, frame * 9) * Math.PI / 180) * 254 * mul;
    const spread = (254 * mul - y) * 0.3;
    const bottom = 277 * scale.sy + 127 * mul;
    if (images.card && y > 0) texturedQuad(ctx, images.card, [
      [cardRect[0] - spread, bottom - y], [cardRect[0] + cardRect[2] + spread, bottom - y],
      [cardRect[0] + cardRect[2], bottom], [cardRect[0], bottom],
    ]);
  } else if (kind === 'zoom') {
    const diff = Math.min(127, 1 + frame * 9);
    ctx.globalAlpha = Math.min(255, diff * 2) / 255;
    if (images.card) ctx.drawImage(images.card, ...rect(662 - diff * 0.69685, 277 - diff, 662 + diff * 0.69685, 277 + diff));
  } else if (kind === 'fade') {
    ctx.globalAlpha = Math.min(255, frame * 17) / 255;
    card();
  } else {
    card();
    if (kind === 'reveal' && images.mask) {
      if (frame < 17) {
        const diff = frame * 15, width = Math.min(177, diff);
        if (width) ctx.drawImage(images.mask, 254 - diff, 0, width, 254, ...rect(574, 150, 574 + width, 404));
      } else {
        const diff = (frame - 17) * 15;
        if (diff < 177) ctx.drawImage(images.mask, 0, 0, 177 - diff, 254, ...rect(574 + diff, 150, 751, 404));
      }
    } else if (kind === 'negated' || kind === 'number') {
      const diff = Math.min(64, frame * 4);
      const dest = rect(536 + diff, 141 + diff, 792 - diff, 397 - diff);
      if (kind === 'negated' && images.negated) ctx.drawImage(images.negated, 0, 0, 128, 128, ...dest);
      if (kind === 'number' && images.number) {
        const index = Math.max(0, num - 1);
        ctx.drawImage(images.number, (index % 5) * 64, Math.floor(index / 5) * 64, 64, 64, ...dest);
      }
    }
  }
  ctx.globalAlpha = 1;
}

export function rpsOffset(frame: number): number {
  let offset = 50;
  for (let i = 0; i < Math.min(frame, 29); i++) offset += Math.trunc(-0.333333 * i + 10);
  return offset;
}
