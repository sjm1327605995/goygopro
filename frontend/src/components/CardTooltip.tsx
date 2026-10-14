/**
 * gframe stTip: white 0xc0 block above/left of the pointer, with live card
 * attack/defense and grade/race/attribute lines. Selection hints remain in
 * stHintMsg. The event carries physical pointer coordinates outside DesignSpace.
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
  useLayoutEffect(()=>{
    const el=tipRef.current;if(el&&tip){el.style.left=`${tip.x-el.offsetWidth-10}px`;el.style.top=`${tip.y-el.offsetHeight-10}px`;}
  },[tip]);

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
      style={{
        position: 'fixed', left: `${x}px`, top: `${y}px`, zIndex: 1200,
        background: 'rgba(255,255,255,0.75)', color: '#000',
        padding: '5px', fontSize: '12px', fontWeight: 400,
        pointerEvents: 'none',whiteSpace:'pre',textAlign:'center',
      }}
    >
      <div>{tip.text}</div>
      {tip.lines.map((line, i) => (
        <div key={i} style={{ fontWeight: 400, color: '#525252' }}>{line}</div>
      ))}
    </div>
  );
}
