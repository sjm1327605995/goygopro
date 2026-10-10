import { useSyncExternalStore } from 'react';
import { duelStore } from '../../duel/store.ts';
import { CardSelectModal } from './CardSelectModal.tsx';
import type { SelectCard } from './CardTile.tsx';

/**
 * store.cardSelect 驱动的卡片选择弹窗（select_card / select_unselect）：
 * 选择集活在 store（与场上 3D 点选共用），store 变化时本组件重渲染，
 * 因此弹窗里的点选与场上高亮点击天然同步。选择态清空（应答/取消）时
 * 返回 null，由 PromptHost 的订阅顺带关掉弹窗壳。
 */
export function StoreCardSelectModal({ title, cards, respond }: {
  title: string;
  cards: SelectCard[];
  respond: (result: { indices: number[] | null }) => void;
}) {
  const cs = useSyncExternalStore(duelStore.subscribe, duelStore.getState).cardSelect;
  if (!cs) return null;
  // 位置标签（原版 stCardPos）需要区域信息：named 只有 code/name，
  // 从 cs.cards 按序并入 l/s
  const withLoc = cards.map((c, i) => ({ ...c, l: cs.cards[i]?.l, s: cs.cards[i]?.s }));
  return (
    <CardSelectModal
      title={title}
      cards={withLoc}
      min={cs.min}
      max={cs.max}
      cancelable={cs.cancelable}
      external={{
        selected: cs.selected,
        toggle: (idx: number) => duelStore.toggleCardSelect(idx),
      }}
      respond={respond}
    />
  );
}
