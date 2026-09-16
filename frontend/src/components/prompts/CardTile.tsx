import React, { useEffect, useRef } from 'react';
import { WailsBridge } from '../../wails_bridge.ts';

/** 可选卡片的公共形状；index signature 让 SumSelect 的 param 等附加字段透传 */
export interface SelectCard {
  code: number;
  name: string;
  [k: string]: unknown;
}

/** 卡牌图块：cover 兜底 + 异步真实卡图（旧 fillCardTileImages 的 React 版） */
export function CardTile({ code, name, className = '', children, onPick, dataAttrs }: {
  code: number;
  name: string;
  className?: string;
  children?: React.ReactNode;
  onPick?: () => void;
  dataAttrs?: Record<string, string | number>;
}) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    let alive = true;
    if (!code) return undefined;
    WailsBridge.getCardImage(code).then((pic) => {
      if (!alive || !pic || !pic.url || !ref.current) return;
      ref.current.style.backgroundImage = `url("${pic.url}")`;
      ref.current.classList.add('card-tile-loaded');
    }).catch(() => {});
    return () => { alive = false; };
  }, [code]);

  return (
    <div
      ref={ref}
      className={`select-card-item card-tile ${className}`}
      data-code={code}
      {...(dataAttrs || {})}
      onClick={onPick}
    >
      {children}
      <div className="card-tile-name">{name}</div>
    </div>
  );
}