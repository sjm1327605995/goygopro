import { CardTile } from './CardTile.tsx';
import type { SelectCard } from './CardTile.tsx';
import { useCardSelection } from './useCardSelection.ts';
import { sysString } from '../../domain/sys_strings.ts';

// 原版 wCardSelect/wCardDisplay（game.cpp:578-600）：窗口 (320,100)-(1000,400)
// 680×300；每卡一列 = 位置标签 stCardPos (30+125i,30) 120×20 白底 +
// 卡图钮 btnCardSelect (30+125i,55) 120×170；>5 张横向滚动（原版 scrCardList 翻页）。
// 位置标签用 ocgcore LOCATION → SysString 1000+ 区域名。
const ZONE_STR_ID: Record<number, number> = {
  0x01: 1000, 0x02: 1001, 0x04: 1002, 0x08: 1003, 0x10: 1004, 0x20: 1005, 0x40: 1006, 0x80: 1007,
};
const zoneLabel = (loc: unknown): string => {
  const id = ZONE_STR_ID[Number(loc) || 0];
  return id ? (sysString(id) || '') : '';
};

export function CardSelectModal({ title, cards, min, max, cancelable, respond, external }: {
  title: string;
  cards: SelectCard[];
  min: number;
  max: number;
  cancelable: boolean;
  respond: (result: { indices: number[] | null }) => void;
  /**
   * 外置选择集（store.cardSelect 驱动）：select_card/select_unselect 的
   * 弹窗与场上 3D 点选共用同一份 selected，保证两条路径状态同步；
   * 不传则沿用本地 useCardSelection（option/chain 等单源弹窗）。
   */
  external?: { selected: number[]; toggle: (idx: number) => void };
}) {
  const local = useCardSelection(min, max);
  const selected = external ? external.selected : local.selected;
  const toggle = external ? external.toggle : local.toggle;
  const minMet = selected.length >= min;
  const done = (indices: number[] | null) => respond({ indices });

  return (
    <div className="modal-box wcs-window">
      <div className="modal-title">{`${title}（选择 ${min}-${max} 张）`}</div>
      <div className="wcs-cardrow">
        {cards.map((c, idx) => (
          <div className="wcs-cell" key={idx}>
            <div className="wcs-pos">{zoneLabel(c.l)}</div>
            <CardTile
              code={c.code}
              name={c.name}
              dataAttrs={{ 'data-idx': idx }}
              className={selected.includes(idx) ? 'selected' : ''}
              onPick={() => toggle(idx)}
            />
          </div>
        ))}
      </div>
      <div className="wcs-footer">
        <span id="select-count-text" style={{ fontSize: '13px', color: 'var(--text-muted)' }}>
          {`已选 ${selected.length} / ${max} 张`}
        </span>
        <div style={{ display: 'flex', gap: '8px' }}>
          {cancelable && (
            <button id="modal-select-cancel" className="btn btn-secondary" onClick={() => done(null)}>取消</button>
          )}
          <button
            id="modal-select-confirm"
            className={`btn btn-gold${minMet ? ' btn-flash-gold' : ''}`}
            disabled={!minMet}
            onClick={() => done(selected)}
          >确定</button>
        </div>
      </div>
    </div>
  );
}
