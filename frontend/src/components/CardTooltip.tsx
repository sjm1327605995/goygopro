/**
 * gframe stTip: white 0xc0 block above/left of the pointer, flipping only when
 * it would leave the viewport. Shows live card attack/defense and grade/race/
 * attribute lines. The event carries physical pointer coordinates outside DesignSpace.
 */
import React, { useEffect, useLayoutEffect, useRef, useState } from 'react';
import {TYPE_MONSTER,TYPE_LINK,TYPE_XYZ,TYPE_PENDULUM,formatRace,formatAttribute} from '../domain/constants.ts';

interface HoverDetail {
  location?:string;
  code?: number;
  info?: any;
  live?: {
    attack?: number; defense?: number;
    level?: number; rank?: number; link?: number;
  };
  x?: number;
  y?: number;
}

interface TipState {
  text: string;
  lines: string[];
  x: number;
  y: number;
}

function buildLines(info: any, live: HoverDetail['live'],location?:string): string[] {
  if (!info || typeof info.type !== 'number') return [];
  const lines: string[] = [];
  if (info.type & TYPE_MONSTER) {
    const fmt = (v: any) => (typeof v === 'number' && v >= 0 ? String(v) : '?');
    const atk = live?.attack ?? info.attack;
    const def = live?.defense ?? info.defense;
    const race=formatRace(info.race||0),attr=formatAttribute(info.attribute||0);
    if (info.type & TYPE_LINK) {
      lines.push(`${fmt(atk)}/LINK-${live?.link ?? info.level ?? '?'}`);
      lines.push(`LINK-${live?.link??info.level??'?'} ${race}/${attr}`);
    } else {
      lines.push(`${fmt(atk)}/${fmt(def)}`);
      const lv = live?.level ?? info.level;
      if (info.type & TYPE_XYZ) lines.push(`☆${live?.rank ?? lv ?? '?'} ${race}/${attr}`);
      else lines.push(`★${lv ?? '?'} ${race}/${attr}`);
    }
    if (location==='hand'&&(info.type & TYPE_PENDULUM) && info.lscale !== undefined) {
      lines.push(`${info.lscale}/${info.rscale}`);
    }
  } else {
    if(location==='szone'&&(info.type&TYPE_PENDULUM))lines.push(`${info.lscale}/${info.rscale}`);
  }
  return lines;
}

export default function CardTooltip() {
  const [tip, setTip] = useState<TipState | null>(null);
  const tipRef = useRef<HTMLDivElement | null>(null);
  useLayoutEffect(() => {
    const el = tipRef.current;
    if (!el || !tip) return undefined;
    const update = () => {
      const margin = 8;
      const width = el.offsetWidth;
      const height = el.offsetHeight;
      let x = tip.x - width - 10;
      let y = tip.y - height - 10;
      if (x < margin) x = tip.x + 10;
      if (y < margin) y = tip.y + 10;
      x = Math.max(margin, Math.min(x, window.innerWidth - width - margin));
      y = Math.max(margin, Math.min(y, window.innerHeight - height - margin));
      el.style.left = `${x}px`;
      el.style.top = `${y}px`;
    };
    update();
    window.addEventListener('resize', update);
    return () => window.removeEventListener('resize', update);
  }, [tip]);

  useEffect(() => {
    const container = document.getElementById('duel-canvas-container');
    if (!container) return;

    const onHover = (ev: Event) => {
      const d = (ev as CustomEvent<HoverDetail>).detail || {};
      const text = d.info && d.info.name ? d.info.name : (d.code ? String(d.code) : '');
      if (!text) {setTip(null);return;}
      setTip({ text, lines: buildLines(d.info, d.live,d.location), x: d.x || 0, y: d.y || 0 });
    };
    const onHoverEnd = () => setTip(null);

    container.addEventListener('ygo:cardhover', onHover);
    container.addEventListener('ygo:cardhoverend', onHoverEnd);
    return () => {
      container.removeEventListener('ygo:cardhover', onHover);
      container.removeEventListener('ygo:cardhoverend', onHoverEnd);
    };
  }, []);

  if (!tip) return null;

  // event_handler.cpp:1659: measured tip ends 10px above/left of pointer.
  const w = tipRef.current ? tipRef.current.offsetWidth : 150;
  const h = tipRef.current ? tipRef.current.offsetHeight : 28;
  const x = tip.x-w-10;
  const y = tip.y-h-10;

  return (
    <div
      ref={tipRef}
      id="st-tip"
      role="tooltip"
      className="st-tip"
      style={{
        position: 'fixed', left: `${x}px`, top: `${y}px`, zIndex: 1200,
        pointerEvents: 'none', whiteSpace: 'pre-wrap', overflowWrap: 'anywhere',
        maxWidth: 'calc(100vw - 16px)', boxSizing: 'border-box', textAlign: 'center',
      }}
    >
      <div>{tip.text}</div>
      {tip.lines.map((line, i) => (
        <div key={i} className="st-tip-line">{line}</div>
      ))}
    </div>
  );
}
