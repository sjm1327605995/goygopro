import React, { useEffect, useRef, useState } from 'react';

// 原版 GUI 坐标系：1024×640 设计分辨率，窗口缩放时按
// Resize(x) = x * winW/1024、y * winH/640 非均匀整体拉伸
//（game.cpp Resize / ResizePhaseHint 的语义）。此组件提供这块画布：
// 子元素一律用设计像素 absolute 定位，画布随视口拉伸。
export const DESIGN_W = 1024;
export const DESIGN_H = 640;

export default function DesignSpace({ children }: { children: React.ReactNode }) {
  const viewportRef = useRef<HTMLDivElement>(null);
  const [scale, setScale] = useState({ sx: 1, sy: 1 });

  useEffect(() => {
    const el = viewportRef.current;
    if (!el) return;
    const update = () => {
      const r = el.getBoundingClientRect();
      setScale({ sx: r.width / DESIGN_W, sy: r.height / DESIGN_H });
    };
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  return (
    <div ref={viewportRef} className="design-space-viewport">
      <div
        className="design-space"
        style={{ transform: `scale(${scale.sx}, ${scale.sy})` }}
      >
        {children}
      </div>
    </div>
  );
}
