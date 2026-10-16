import React, { useContext } from 'react';
import { DesignScaleContext } from './DesignSpace.tsx';

export function useSourceWindow(x: number, y: number, width: number, height: number): React.CSSProperties {
  const { sx, sy } = useContext(DesignScaleContext);
  return { position: 'fixed', left: x + width / 2, top: y + height / 2, width, height,
    transformOrigin: '0 0', transform: `scale(${1 / sx}, ${1 / sy}) translate(${-width / 2}px, ${-height / 2}px)` };
}

export default function SourceWindow({ x, y, width, height, style, ...props }: React.HTMLAttributes<HTMLDivElement> & {
  x: number; y: number; width: number; height: number;
}) {
  const geometry = useSourceWindow(x, y, width, height);
  return <div {...props} style={{ ...style, ...geometry }} />;
}
