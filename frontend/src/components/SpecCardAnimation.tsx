import { useContext, useEffect, useRef } from 'react';
import { DesignScaleContext } from '../ui/DesignSpace.tsx';
import { drawSpecFrame, type SpecKind } from '../duel/draw_spec.ts';

const cache = new Map<string, Promise<HTMLImageElement | null>>();
function loadImage(url: string): Promise<HTMLImageElement | null> {
  let pending = cache.get(url);
  if (!pending) {
    pending = new Promise(resolve => {
      const img = new Image();
      img.onload = () => resolve(img);
      img.onerror = () => resolve(null);
      img.src = url;
    });
    cache.set(url, pending);
  }
  return pending;
}

export default function SpecCardAnimation({ kind, url, num, onComplete }: {
  kind: SpecKind; url: string | null; num?: number; onComplete: () => void;
}) {
  const scale = useContext(DesignScaleContext);
  const canvas = useRef<HTMLCanvasElement>(null);
  const done = useRef(onComplete);
  done.current = onComplete;
  const dimensions = useRef(scale);
  dimensions.current = scale;
  useEffect(() => {
    let alive = true, raf = 0;
    Promise.all([url ? loadImage(url) : null, loadImage('textures/mask.png'),
      loadImage('textures/negated.png'), loadImage('textures/number.png')]).then(([card, mask, negated, number]) => {
      if (!alive) return;
      const start = performance.now();
      const tick = (now: number) => {
        if (!alive) return;
        const frame = Math.floor((now - start) * 60 / 1000);
        if (frame >= 30) { done.current(); return; }
        const el = canvas.current;
        const ctx = el?.getContext('2d');
        if (el && ctx) {
          const dpr = window.devicePixelRatio || 1;
          const sx = dimensions.current.sx * dpr, sy = dimensions.current.sy * dpr;
          const width = Math.round(1024 * sx), height = Math.round(640 * sy);
          if (el.width !== width || el.height !== height) { el.width = width; el.height = height; }
          drawSpecFrame(ctx, kind, frame, { card, mask, negated, number }, { sx, sy }, num);
          el.dataset.frame = String(frame);
        }
        raf = requestAnimationFrame(tick);
      };
      raf = requestAnimationFrame(tick);
    });
    return () => { alive = false; cancelAnimationFrame(raf); };
  }, [kind, url, num]);
  return <canvas ref={canvas} className={kind === 'reveal' ? 'spec-mask' : undefined}
    aria-label={kind === 'number' ? `回合数 ${num}` : '卡片特效'} style={{ width: '100%', height: '100%' }}>{kind === 'number' ? num : ''}</canvas>;
}
