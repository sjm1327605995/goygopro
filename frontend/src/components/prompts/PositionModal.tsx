import { useEffect, useState } from 'react';
import { WailsBridge } from '../../wails_bridge.ts';
import {
  POS_FACEUP_ATTACK, POS_FACEDOWN_ATTACK, POS_FACEUP_DEFENSE, POS_FACEDOWN_DEFENSE,
} from '../../domain/constants.ts';

/** 表示形式选择（原版 wPosSelect，game.cpp:567-575）：137×137 方形按钮，
 *  表侧真实卡图（守备旋转 90°），里侧卡背（gframe duelclient.cpp:1920-1931） */
export function PositionModal({ positions, code, respond }: { positions: number; code: number; respond: (pos: number) => void }) {
  const [pic, setPic] = useState<string | null>(null);
  useEffect(() => {
    let alive = true;
    if (!code) return undefined;
    WailsBridge.getCardImage(code).then((p) => {
      if (alive && p && p.url) setPic(p.url);
    }).catch(() => {});
    return () => { alive = false; };
  }, [code]);

  const options: [number, string, boolean, boolean][] = [];
  if (positions & POS_FACEUP_ATTACK) options.push([POS_FACEUP_ATTACK, '表侧攻击表示', false, false]);
  if (positions & POS_FACEDOWN_ATTACK) options.push([POS_FACEDOWN_ATTACK, '里侧攻击表示', true, false]);
  if (positions & POS_FACEUP_DEFENSE) options.push([POS_FACEUP_DEFENSE, '表侧守备表示', false, true]);
  if (positions & POS_FACEDOWN_DEFENSE) options.push([POS_FACEDOWN_DEFENSE, '里侧守备表示', true, true]);
  if (!options.length) {
    options.push([POS_FACEUP_ATTACK, '表侧攻击表示', false, false], [POS_FACEDOWN_DEFENSE, '里侧守备表示', true, true]);
  }

  return (
    <div className="modal-box modal-pos">
      <div className="modal-title">请选择表示形式</div>
      <div className="pos-row">
        {options.map(([pos, label, facedown, rotated], i) => (
          <button
            key={pos}
            data-pos={pos}
            className="pos-opt"
            id={i === 0 ? 'pos-first-btn' : undefined}
            title={label}
            onClick={() => respond(pos)}
          >
            <span
              className={`pos-opt-img${facedown ? ' pos-opt-facedown' : ''}${rotated ? ' pos-opt-rot' : ''}`}
              style={pic && !facedown ? { backgroundImage: `url("${pic}")` } : undefined}
            />
          </button>
        ))}
      </div>
    </div>
  );
}