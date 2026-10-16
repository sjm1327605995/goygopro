import React, { useContext } from 'react';
import { DesignScaleContext } from './DesignSpace.tsx';

// game.cpp ResizeCardImgWin: scale the card uniformly, padding with each axis.
export default function CardImageFrame({ className, imageClassName = '', children }: {
  className: string; imageClassName?: string; children: React.ReactNode;
}) {
  const { sx, sy } = useContext(DesignScaleContext);
  const mul = Math.min(sx, sy);
  return <div className={className} style={{ position: 'fixed', left: Math.trunc(sx) / sx, top: Math.trunc(sy) / sy,
    width: Math.trunc(177 * mul + 20 * sx), height: Math.trunc(254 * mul + 18 * sy),
    transformOrigin: '0 0', transform: `scale(${1 / sx}, ${1 / sy})` }}>
    <div className={`native-card-image ${imageClassName}`} style={{ position: 'absolute',
      left: Math.trunc(10 * sx) - 1, top: Math.trunc(9 * sy) - 1,
      width: Math.trunc(177 * mul), height: Math.trunc(254 * mul), padding: 0, border: 0 }}>
      {children}
    </div>
  </div>;
}
